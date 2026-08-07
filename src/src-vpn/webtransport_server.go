// File: webtransport_server.go
// WebTransport (QUIC) server — per architecture table T2 (winner: 166 pts).
// PWA VPN transport layer. Replaces WireGuard/SOCKS5 for PWA.
package vpn

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
)

// WTServer is a WebTransport server that accepts PWA VPN connections.
type WTServer struct {
	listener  *quic.Listener
	tlsConfig *tls.Config
	addr      string
	stopOnce  sync.Once
	stopChan  chan struct{}
	clients   sync.Map
	nextID    atomic.Uint64
	byteIn    atomic.Uint64
	byteOut   atomic.Uint64
	startedAt time.Time
}

// WTClient represents a connected WebTransport client.
type WTClient struct {
	ID          uint64
	Session     *quic.Conn
	RemoteAddr  string
	ConnectedAt time.Time
}

// NewWTServer creates a new WebTransport server with self-signed TLS cert.
func NewWTServer(addr string) (*WTServer, error) {
	tlsConfig, err := generateSelfSignedTLS()
	if err != nil {
		return nil, fmt.Errorf("wt tls: %w", err)
	}
	return &WTServer{
		tlsConfig: tlsConfig,
		addr:      addr,
		stopChan:  make(chan struct{}),
	}, nil
}

// Start begins listening for QUIC connections.
func (s *WTServer) Start() error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return fmt.Errorf("wt resolve: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("wt listen udp: %w", err)
	}

	quicConfig := &quic.Config{
		MaxIdleTimeout:        30 * time.Second,
		MaxIncomingStreams:    100,
		MaxIncomingUniStreams: 100,
		KeepAlivePeriod:       10 * time.Second,
	}

	listener, err := quic.Listen(udpConn, s.tlsConfig, quicConfig)
	if err != nil {
		udpConn.Close()
		return fmt.Errorf("wt quic listen: %w", err)
	}

	s.listener = listener
	s.startedAt = time.Now()
	fmt.Printf("[WT] WebTransport server listening on %s (QUIC)\n", s.addr)

	go s.acceptLoop()
	return nil
}

func (s *WTServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept(context.Background())
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
				continue
			}
		}
		go s.handleConnection(conn)
	}
}

func (s *WTServer) handleConnection(conn *quic.Conn) {
	clientID := s.nextID.Add(1)
	client := &WTClient{
		ID:          clientID,
		Session:     conn,
		RemoteAddr:  conn.RemoteAddr().String(),
		ConnectedAt: time.Now(),
	}
	s.clients.Store(clientID, client)
	defer s.clients.Delete(clientID)

	fmt.Printf("[WT] Client %d connected from %s\n", clientID, client.RemoteAddr)

	ctx := conn.Context()
	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			return
		}
		go s.handleStream(stream, clientID)
	}
}

// Wire protocol: [1 byte type][4 bytes length][payload]
// Type 0x01 = CONNECT, 0x02 = DATA, 0x03 = CLOSE
func (s *WTServer) handleStream(stream *quic.Stream, clientID uint64) {
	defer stream.Close()

	for {
		header := make([]byte, 5)
		if _, err := io.ReadFull(stream, header); err != nil {
			return
		}

		msgType := header[0]
		bodyLen := binary.BigEndian.Uint32(header[1:5])
		if bodyLen > 65536 {
			return
		}

		body := make([]byte, bodyLen)
		if _, err := io.ReadFull(stream, body); err != nil {
			return
		}

		s.byteIn.Add(uint64(len(body) + 5))

		switch msgType {
		case 0x01: // CONNECT
			target := string(body)
			fmt.Printf("[WT] Client %d CONNECT -> %s\n", clientID, target)

			targetConn, err := net.DialTimeout("tcp", target, 10*time.Second)
			if err != nil {
				s.sendError(stream, fmt.Sprintf("connect failed: %v", err))
				continue
			}

			s.sendHeader(stream, 0x81, []byte("ok"))

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				io.Copy(targetConn, stream)
				if tc, ok := targetConn.(*net.TCPConn); ok {
					tc.CloseWrite()
				}
			}()

			go func() {
				defer wg.Done()
				n, _ := io.Copy(stream, targetConn)
				s.byteOut.Add(uint64(n))
			}()

			wg.Wait()
			targetConn.Close()
			return

		case 0x03:
			return

		default:
			continue
		}
	}
}

func (s *WTServer) sendHeader(stream *quic.Stream, msgType byte, payload []byte) {
	header := make([]byte, 5)
	header[0] = msgType
	binary.BigEndian.PutUint32(header[1:5], uint32(len(payload)))
	stream.Write(header)
	if len(payload) > 0 {
		stream.Write(payload)
	}
}

func (s *WTServer) sendError(stream *quic.Stream, msg string) {
	s.sendHeader(stream, 0xFF, []byte(msg))
}

func (s *WTServer) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopChan)
		if s.listener != nil {
			l := *s.listener
			l.Close()
		}
		fmt.Println("[WT] Server stopped")
	})
}

func (s *WTServer) GetCertHash() string {
	if s.tlsConfig == nil || len(s.tlsConfig.Certificates) == 0 {
		return ""
	}
	cert := s.tlsConfig.Certificates[0].Certificate[0]
	x509cert, err := x509.ParseCertificate(cert)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(x509cert.SerialNumber.Bytes())
}

func (s *WTServer) GetStats() map[string]interface{} {
	var clientCount int
	s.clients.Range(func(_, _ interface{}) bool {
		clientCount++
		return true
	})
	return map[string]interface{}{
		"transport": "webtransport",
		"addr":      s.addr,
		"clients":   clientCount,
		"bytesIn":   s.byteIn.Load(),
		"bytesOut":  s.byteOut.Load(),
		"uptime":    int(time.Since(s.startedAt).Seconds()),
		"certHash":  s.GetCertHash(),
	}
}

func generateSelfSignedTLS() (*tls.Config, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("ecdsa key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{Organization: []string{"IndestructibleVPN"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("x509 cert: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  priv,
		}},
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{"h3"},
	}, nil
}
// LocalAddr returns the actual listening address of the server.
func (s *WTServer) LocalAddr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}


