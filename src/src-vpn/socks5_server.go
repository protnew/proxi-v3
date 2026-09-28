package vpn

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// SOCKS5Server is a real local SOCKS5 proxy. Traffic dials out from this process
// (optionally chained through an upstream SOCKS5). This is the Windows-testable
// "real VPN" path without kernel WireGuard/Wintun.
type SOCKS5Server struct {
	listenAddr    string
	upstreamSOCKS string // empty = direct egress; else host:port upstream SOCKS5
	chain         *Chain // D1: when set, dial goes DC→WT→userspace→Tor→Nostr, not the NIC
	ln            net.Listener
	running       atomic.Bool
	bytesIn       atomic.Int64
	bytesOut      atomic.Int64
	conns         atomic.Int64
	mu            sync.Mutex
	active        map[net.Conn]struct{}
}

func NewSOCKS5Server(listenAddr, upstreamSOCKS string) *SOCKS5Server {
	if listenAddr == "" {
		listenAddr = "127.0.0.1:10808"
	}
	return &SOCKS5Server{
		listenAddr:    listenAddr,
		upstreamSOCKS: upstreamSOCKS,
		active:        make(map[net.Conn]struct{}),
	}
}

func (s *SOCKS5Server) Addr() string     { return s.listenAddr }
func (s *SOCKS5Server) Upstream() string { return s.upstreamSOCKS }
func (s *SOCKS5Server) IsRunning() bool  { return s.running.Load() }
func (s *SOCKS5Server) Stats() (in, out, conns int64) {
	return s.bytesIn.Load(), s.bytesOut.Load(), s.conns.Load()
}

func (s *SOCKS5Server) Start() error {
	if s.running.Load() {
		return nil
	}
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("socks5 listen %s: %w", s.listenAddr, err)
	}
	s.ln = ln
	s.running.Store(true)
	go s.acceptLoop()
	fmt.Printf("[VPN] SOCKS5 listening on %s (upstream=%q)\n", s.listenAddr, s.upstreamSOCKS)
	return nil
}

func (s *SOCKS5Server) Stop() error {
	if !s.running.Swap(false) {
		return nil
	}
	if s.ln != nil {
		_ = s.ln.Close()
	}
	s.mu.Lock()
	for c := range s.active {
		_ = c.Close()
	}
	s.active = make(map[net.Conn]struct{})
	s.mu.Unlock()
	return nil
}

func (s *SOCKS5Server) acceptLoop() {
	for s.running.Load() {
		c, err := s.ln.Accept()
		if err != nil {
			if !s.running.Load() {
				return
			}
			continue
		}
		s.mu.Lock()
		s.active[c] = struct{}{}
		s.mu.Unlock()
		s.conns.Add(1)
		go s.handle(c)
	}
}

func (s *SOCKS5Server) untrack(c net.Conn) {
	s.mu.Lock()
	delete(s.active, c)
	s.mu.Unlock()
	_ = c.Close()
}

func (s *SOCKS5Server) handle(client net.Conn) {
	defer s.untrack(client)
	_ = client.SetDeadline(time.Now().Add(60 * time.Second))

	// greeting
	buf := make([]byte, 258)
	if _, err := io.ReadFull(client, buf[:2]); err != nil {
		return
	}
	if buf[0] != 0x05 {
		return
	}
	nMethods := int(buf[1])
	if nMethods <= 0 || nMethods > 255 {
		return
	}
	if _, err := io.ReadFull(client, buf[:nMethods]); err != nil {
		return
	}
	// no auth
	if _, err := client.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	// request
	if _, err := io.ReadFull(client, buf[:4]); err != nil {
		return
	}
	if buf[0] != 0x05 || buf[1] != 0x01 { // CONNECT only
		s.reply(client, 0x07) // command not supported
		return
	}
	atyp := buf[3]
	var host string
	switch atyp {
	case 0x01: // IPv4
		if _, err := io.ReadFull(client, buf[:4]); err != nil {
			return
		}
		host = net.IP(buf[:4]).String()
	case 0x03: // domain
		if _, err := io.ReadFull(client, buf[:1]); err != nil {
			return
		}
		l := int(buf[0])
		if _, err := io.ReadFull(client, buf[:l]); err != nil {
			return
		}
		host = string(buf[:l])
	case 0x04: // IPv6
		if _, err := io.ReadFull(client, buf[:16]); err != nil {
			return
		}
		host = net.IP(buf[:16]).String()
	default:
		s.reply(client, 0x08)
		return
	}
	if _, err := io.ReadFull(client, buf[:2]); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(buf[:2])
	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	remote, err := s.dialTarget(target)
	if err != nil {
		s.reply(client, 0x05) // connection refused
		return
	}
	defer remote.Close()

	// success reply: bind 0.0.0.0:0
	if _, err := client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	_ = client.SetDeadline(time.Time{})
	_ = remote.SetDeadline(time.Time{})

	// bidirectional copy with counters
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n, _ := io.Copy(remote, client)
		s.bytesOut.Add(n)
		_ = remote.Close()
	}()
	go func() {
		defer wg.Done()
		n, _ := io.Copy(client, remote)
		s.bytesIn.Add(n)
		_ = client.Close()
	}()
	wg.Wait()
}

func (s *SOCKS5Server) reply(c net.Conn, code byte) {
	_, _ = c.Write([]byte{0x05, code, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
}

// UseChain attaches the D1 transport chain. Stage 0 stays SOCKS; the chain is the upstream.
func (s *SOCKS5Server) UseChain(c *Chain) { s.chain = c }

func (s *SOCKS5Server) dialTarget(target string) (net.Conn, error) {
	if s.chain != nil {
		conn, _, err := s.chain.Dial(context.Background(), target)
		return conn, err
	}
	d := net.Dialer{Timeout: 20 * time.Second}
	if s.upstreamSOCKS == "" {
		return d.Dial("tcp", target)
	}
	return dialViaSOCKS5(s.upstreamSOCKS, target, 20*time.Second)
}

// dialViaSOCKS5 opens target through an upstream SOCKS5 proxy.
func dialViaSOCKS5(proxyAddr, target string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("upstream socks %s: %w", proxyAddr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		conn.Close()
		return nil, err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil || resp[0] != 0x05 || resp[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("upstream socks auth failed")
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		conn.Close()
		return nil, err
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, []byte(host)...)
	req = append(req, byte(port>>8), byte(port&0xff))
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}
	rbuf := make([]byte, 256)
	n, err := conn.Read(rbuf)
	if err != nil || n < 2 || rbuf[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("upstream socks connect failed")
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}
