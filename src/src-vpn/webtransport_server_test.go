package vpn

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

// TestWTServerStartStop verifies the WebTransport server can start and stop.
func TestWTServerStartStop(t *testing.T) {
	srv, err := NewWTServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewWTServer: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop()

	// Verify stats
	stats := srv.GetStats()
	if stats["transport"] != "webtransport" {
		t.Errorf("expected transport=webtransport, got %v", stats["transport"])
	}
	if stats["certHash"] == "" {
		t.Error("expected non-empty certHash")
	}
}

// TestWTServerQUICConnect verifies a QUIC client can connect to the WT server.
func TestWTServerQUICConnect(t *testing.T) {
	srv, err := NewWTServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewWTServer: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop()

	// Get actual listening address
	addr := srv.LocalAddr()

	// Connect a QUIC client
	tr := &quic.Transport{
		Conn: mustListenUDP(t),
	}
	defer tr.Close()

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
		MinVersion:         tls.VersionTLS13,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Extract port
	_, port, _ := net.SplitHostPort(addr)
	conn, err := tr.Dial(ctx, mustResolveUDP(t, "127.0.0.1:"+port), tlsConf, &quic.Config{MaxIdleTimeout: 10 * time.Second, KeepAlivePeriod: 5 * time.Second})
	if err != nil {
		t.Fatalf("QUIC dial: %v", err)
	}
	defer conn.CloseWithError(0, "")

	// Open a bi-directional stream and send CONNECT
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}

	// Start a TCP echo server to test CONNECT
	echoAddr := startTCPEcho(t)

	// Send CONNECT frame: type=0x01, len, payload=echoAddr
	target := []byte(echoAddr)
	header := make([]byte, 5)
	header[0] = 0x01
	binary.BigEndian.PutUint32(header[1:5], uint32(len(target)))
	stream.Write(header)
	stream.Write(target)

	// Read response header (0x81 = connect ok)
	respHeader := make([]byte, 5)
	if _, err := io.ReadFull(stream, respHeader); err != nil {
		t.Fatalf("read response header: %v", err)
	}
	if respHeader[0] != 0x81 {
		respBody := make([]byte, binary.BigEndian.Uint32(respHeader[1:5]))
		io.ReadFull(stream, respBody)
		t.Fatalf("expected 0x81 (connect ok), got 0x%02x: %s", respHeader[0], string(respBody))
	}

	// Read response body length
	respLen := binary.BigEndian.Uint32(respHeader[1:5])
	if respLen > 0 {
		respBody := make([]byte, respLen)
		io.ReadFull(stream, respBody)
	}

	// Send data through the tunnel — should echo back
	testData := []byte("HELLO_WEBTRANSPORT_VPN")
	stream.Write(testData)

	// Read echoed data
	echoed := make([]byte, len(testData))
	stream.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(stream, echoed)
	if err != nil {
		t.Fatalf("read echoed data: %v", err)
	}
	stream.SetReadDeadline(time.Time{})

	if string(echoed) != string(testData) {
		t.Errorf("echo mismatch: sent %q, got %q", testData, echoed)
	}

	t.Logf("✅ WebTransport tunnel works! Sent %d bytes, received %d bytes echo", len(testData), len(echoed))
}

// startTCPEcho starts a simple TCP echo server and returns its address.
func startTCPEcho(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) // echo
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func mustListenUDP(t *testing.T) *net.UDPConn {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	return conn
}

func mustResolveUDP(t *testing.T, addr string) *net.UDPAddr {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve udp: %v", err)
	}
	return udpAddr
}

// Avoid unused import warnings
var _ = sync.WaitGroup{}
var _ = fmt.Sprintf
