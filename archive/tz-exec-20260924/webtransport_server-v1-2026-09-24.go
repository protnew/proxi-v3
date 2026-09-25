// File: webtransport_server.go
// Real WebTransport (HTTP/3) exit-node server — architecture table T2.
// Uses github.com/quic-go/webtransport-go so browser WebTransport API can connect.
package vpn

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

const (
	wtMsgConnect   = 0x01
	wtMsgConnectOK = 0x81
	wtMsgError     = 0xFF
)

// WTServer is a WebTransport exit-node.
type WTServer struct {
	addr      string
	tlsConfig *tls.Config
	certDER   []byte
	server    *webtransport.Server
	udpConn   net.PacketConn
	cancel    context.CancelFunc
	started   atomic.Bool
	startTime time.Time
	clients   sync.Map
	bytesIn   atomic.Uint64
	bytesOut  atomic.Uint64
	nextID    atomic.Uint64
	mu        sync.Mutex
}

// NewWTServer creates a WT server on addr (e.g. "127.0.0.1:0" or "0.0.0.0:4433").
func NewWTServer(addr string) (*WTServer, error) {
	tlsConf, certDER, err := generateWTCertificate()
	if err != nil {
		return nil, fmt.Errorf("wt cert: %w", err)
	}
	return &WTServer{addr: addr, tlsConfig: tlsConf, certDER: certDER}, nil
}

func generateWTCertificate() (*tls.Config, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	validFrom := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	validFrom = validFrom.AddDate(0, 0, -int((validFrom.Weekday()+6)%7))
	validUntil := validFrom.Add(13 * 24 * time.Hour)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(now.UnixNano()),
		Subject:               pkix.Name{Organization: []string{"Indestructible VPN"}, CommonName: "wt-exit"},
		NotBefore:             validFrom,
		NotAfter:              validUntil,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.ParseIP("::1")},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	tlsCert := tls.Certificate{Certificate: [][]byte{certDER}, PrivateKey: key}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS13,
	}
	return cfg, certDER, nil
}

// Start begins listening for WebTransport sessions on /wt and /webtransport.
func (s *WTServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started.Load() {
		return nil
	}

	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("wt listen udp: %w", err)
	}
	s.udpConn = udpConn

	h3 := &http3.Server{
		TLSConfig: http3.ConfigureTLSConfig(s.tlsConfig),
		QUICConfig: &quic.Config{
			EnableDatagrams: true,
			MaxIdleTimeout:  5 * time.Minute,
			KeepAlivePeriod: 15 * time.Second,
		},
	}
	webtransport.ConfigureHTTP3Server(h3)

	mux := http.NewServeMux()
	h3.Handler = mux

	wt := &webtransport.Server{
		H3:          h3,
		CheckOrigin: func(*http.Request) bool { return true },
	}
	s.server = wt

	handler := func(w http.ResponseWriter, r *http.Request) {
		sess, err := wt.Upgrade(w, r)
		if err != nil {
			log.Printf("[WT] upgrade failed: %v", err)
			return
		}
		id := s.nextID.Add(1)
		s.clients.Store(id, r.RemoteAddr)
		log.Printf("[WT] session %d from %s", id, r.RemoteAddr)
		go s.handleSession(id, sess)
	}
	mux.HandleFunc("/wt", handler)
	mux.HandleFunc("/webtransport", handler)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.startTime = time.Now()
	s.started.Store(true)

	go func() {
		log.Printf("[WT] WebTransport HTTP/3 listening on %s path=/wt", s.LocalAddr())
		if err := wt.Serve(udpConn); err != nil && ctx.Err() == nil {
			log.Printf("[WT] Serve ended: %v", err)
		}
	}()
	time.Sleep(50 * time.Millisecond)
	return nil
}

func (s *WTServer) handleSession(id uint64, sess *webtransport.Session) {
	defer func() {
		s.clients.Delete(id)
		_ = sess.CloseWithError(0, "bye")
		log.Printf("[WT] session %d closed", id)
	}()
	ctx := sess.Context()
	for {
		stream, err := sess.AcceptStream(ctx)
		if err != nil {
			return
		}
		go s.handleStream(id, stream)
	}
}

func (s *WTServer) handleStream(sessionID uint64, stream *webtransport.Stream) {
	defer func() { _ = stream.Close() }()

	hdr := make([]byte, 5)
	if _, err := io.ReadFull(stream, hdr); err != nil {
		return
	}
	if hdr[0] != wtMsgConnect {
		s.writeFrame(stream, wtMsgError, []byte("unknown type"))
		return
	}
	n := binary.BigEndian.Uint32(hdr[1:5])
	if n == 0 || n > 512 {
		s.writeFrame(stream, wtMsgError, []byte("bad target len"))
		return
	}
	target := make([]byte, n)
	if _, err := io.ReadFull(stream, target); err != nil {
		return
	}
	targetStr := string(target)
	log.Printf("[WT] session %d CONNECT %s", sessionID, targetStr)

	conn, err := net.DialTimeout("tcp", targetStr, 10*time.Second)
	if err != nil {
		s.writeFrame(stream, wtMsgError, []byte(err.Error()))
		return
	}

	s.writeFrame(stream, wtMsgConnectOK, []byte("ok"))

	// Half-close aware bidirectional copy
	errCh := make(chan struct{}, 2)
	go func() {
		defer func() { _ = conn.Close() }()
		copied, _ := io.Copy(conn, stream)
		s.bytesIn.Add(uint64(copied))
		errCh <- struct{}{}
	}()
	go func() {
		copied, _ := io.Copy(stream, conn)
		s.bytesOut.Add(uint64(copied))
		// Close write side of WT stream if supported
		_ = stream.Close()
		errCh <- struct{}{}
	}()
	<-errCh
	<-errCh
}

func (s *WTServer) writeFrame(w io.Writer, typ byte, payload []byte) {
	hdr := make([]byte, 5)
	hdr[0] = typ
	binary.BigEndian.PutUint32(hdr[1:5], uint32(len(payload)))
	_, _ = w.Write(hdr)
	if len(payload) > 0 {
		_, _ = w.Write(payload)
	}
}

// Stop shuts down the server.
func (s *WTServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started.Load() {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.server != nil {
		_ = s.server.Close()
	}
	if s.udpConn != nil {
		_ = s.udpConn.Close()
	}
	s.started.Store(false)
	log.Printf("[WT] Server stopped")
}

// LocalAddr returns the bound UDP address.
func (s *WTServer) LocalAddr() string {
	if s.udpConn == nil {
		return s.addr
	}
	return s.udpConn.LocalAddr().String()
}

// GetCertHash returns hex-encoded SHA-256 of the DER certificate.
func (s *WTServer) GetCertHash() string {
	if len(s.certDER) == 0 {
		return ""
	}
	sum := sha256.Sum256(s.certDER)
	return hex.EncodeToString(sum[:])
}

// GetStats returns runtime stats for HTTP API.
func (s *WTServer) GetStats() map[string]interface{} {
	var n int
	s.clients.Range(func(_, _ interface{}) bool { n++; return true })
	uptime := 0.0
	if s.started.Load() {
		uptime = time.Since(s.startTime).Seconds()
	}
	return map[string]interface{}{
		"transport": "webtransport",
		"running":   s.started.Load(),
		"addr":      s.LocalAddr(),
		"certHash":  s.GetCertHash(),
		"clients":   n,
		"bytesIn":   s.bytesIn.Load(),
		"bytesOut":  s.bytesOut.Load(),
		"uptime":    uptime,
	}
}
