package tor

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewTorDialer(t *testing.T) {
	t.Parallel()

	t.Run("empty address uses default", func(t *testing.T) {
		t.Parallel()
		d := NewTorDialer("")
		if d.proxyAddr != "127.0.0.1:9050" {
			t.Errorf("expected default proxy 127.0.0.1:9050, got %s", d.proxyAddr)
		}
	})

	t.Run("custom address is preserved", func(t *testing.T) {
		t.Parallel()
		d := NewTorDialer("192.168.1.1:9051")
		if d.proxyAddr != "192.168.1.1:9051" {
			t.Errorf("expected proxy 192.168.1.1:9051, got %s", d.proxyAddr)
		}
	})
}

func TestIsTorRunning(t *testing.T) {
	t.Parallel()

	d := NewTorDialer("127.0.0.1:9050")
	running := d.IsTorRunning()
	t.Logf("IsTorRunning = %v (expected false in test env)", running)
	if running {
		t.Log("Tor appears to be running on this machine")
	}
}

func TestIsTorRunning_NotRunning(t *testing.T) {
	d := NewTorDialer("127.0.0.1:1")
	if d.IsTorRunning() {
		t.Error("expected IsTorRunning=false for port 1")
	}
}

func TestGetOnionAddress_Fallback(t *testing.T) {
	t.Parallel()

	addr := GetOnionAddress("/tmp/nonexistent-tor-test-dir-12345")
	if addr != "not-yet-configured.onion" {
		t.Errorf("expected fallback onion address, got %q", addr)
	}
}

func TestGetOnionAddress_EmptyDataDir(t *testing.T) {
	t.Parallel()

	// Empty dataDir should use /var/lib/tor as default
	addr := GetOnionAddress("")
	if addr == "" {
		t.Error("expected non-empty address")
	}
	t.Logf("GetOnionAddress('') = %q", addr)
}

func TestGetOnionAddress_HostnameFile(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := tmpDir + "/hidden_service"
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serviceDir+"/hostname", []byte("testaddress123.onion"), 0644); err != nil {
		t.Fatal(err)
	}

	addr := GetOnionAddress(tmpDir)
	if addr != "testaddress123.onion" {
		t.Errorf("GetOnionAddress = %q, want %q", addr, "testaddress123.onion")
	}
}

func TestGetOnionAddress_HostnameFileWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := tmpDir + "/hidden_service"
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serviceDir+"/hostname", []byte("  trimmed.onion\n  "), 0644); err != nil {
		t.Fatal(err)
	}

	addr := GetOnionAddress(tmpDir)
	if addr != "trimmed.onion" {
		t.Errorf("GetOnionAddress = %q, want %q", addr, "trimmed.onion")
	}
}

func TestGetOnionAddress_HostnameFileEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := tmpDir + "/hidden_service"
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serviceDir+"/hostname", []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	addr := GetOnionAddress(tmpDir)
	// Empty file should be skipped, should hit "proxi" path or fallback
	t.Logf("GetOnionAddress with empty hostname: %q", addr)
}

func TestGetOnionAddress_ProxiHostnameFile(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := tmpDir + "/proxi"
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serviceDir+"/hostname", []byte("proxiservice.onion"), 0644); err != nil {
		t.Fatal(err)
	}

	addr := GetOnionAddress(tmpDir)
	if addr != "proxiservice.onion" {
		t.Errorf("GetOnionAddress = %q, want %q", addr, "proxiservice.onion")
	}
}

func TestHTTPTransport(t *testing.T) {
	t.Parallel()

	d := NewTorDialer("127.0.0.1:9050")
	tr := d.HTTPTransport()
	if tr == nil {
		t.Fatal("HTTPTransport returned nil")
	}
	if tr.DialContext == nil {
		t.Error("Transport.DialContext is nil")
	}
}

func TestHTTPClient(t *testing.T) {
	t.Parallel()

	d := NewTorDialer("127.0.0.1:9050")
	cl := d.HTTPClient()
	if cl == nil {
		t.Fatal("HTTPClient returned nil")
	}
	if cl.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", cl.Timeout, 60*time.Second)
	}
	tr, ok := cl.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	if tr.DialContext == nil {
		t.Error("Transport.DialContext is nil")
	}
}

// startMockSOCKS5 starts a mock SOCKS5 server for testing.
func startMockSOCKS5(t *testing.T, handler func(conn net.Conn)) (*net.TCPListener, string) {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()

	return ln, ln.Addr().String()
}

func TestDial_Success(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		buf := make([]byte, 3)
		if _, err := conn.Read(buf); err != nil {
			return
		}
		if buf[0] != 0x05 || buf[1] != 0x01 || buf[2] != 0x00 {
			t.Errorf("unexpected greeting: %x", buf)
			return
		}
		conn.Write([]byte{0x05, 0x00})

		req := make([]byte, 256)
		n, err := conn.Read(req)
		if err != nil || n < 4 {
			return
		}
		if req[0] != 0x05 || req[1] != 0x01 || req[3] != 0x03 {
			t.Errorf("unexpected connect request: %x", req[:4])
			return
		}

		// Parse domain
		domainLen := int(req[4])
		_ = domainLen

		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		time.Sleep(100 * time.Millisecond)
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	conn, err := d.Dial("tcp", "example.onion:80")
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()
}

func TestDial_VerifyDomainAndPort(t *testing.T) {
	var receivedDomain string
	var receivedPort int

	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})

		req := make([]byte, 256)
		n, _ := conn.Read(req)
		if n >= 7+3 { // minimum for 1-char domain + port
			domainLen := int(req[4])
			if n >= 5+domainLen+2 {
				receivedDomain = string(req[5 : 5+domainLen])
				receivedPort = int(req[5+domainLen])<<8 | int(req[5+domainLen+1])
			}
		}
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		time.Sleep(100 * time.Millisecond)
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	conn, err := d.Dial("tcp", "myhidden.onion:443")
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	conn.Close()

	if receivedDomain != "myhidden.onion" {
		t.Errorf("domain = %q, want %q", receivedDomain, "myhidden.onion")
	}
	if receivedPort != 443 {
		t.Errorf("port = %d, want %d", receivedPort, 443)
	}
}

func TestDialContext_Success(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})
		conn.Read(make([]byte, 256))
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		time.Sleep(100 * time.Millisecond)
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", "test.onion:443")
	if err != nil {
		t.Fatalf("DialContext failed: %v", err)
	}
	defer conn.Close()
}

func TestDial_ProxyUnreachable(t *testing.T) {
	d := NewTorDialer("127.0.0.1:1")
	_, err := d.Dial("tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error when proxy unreachable")
	}
	if !strings.Contains(err.Error(), "connect to Tor proxy") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDialContext_Cancelled(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		time.Sleep(5 * time.Second)
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := d.DialContext(ctx, "tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestDial_BadGreetingResponse(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x04, 0x01})
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	_, err := d.Dial("tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error for bad greeting response")
	}
	if !strings.Contains(err.Error(), "socks5 unexpected response") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDial_ConnectionRefused(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})
		conn.Read(make([]byte, 256))
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	_, err := d.Dial("tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDial_GeneralFailure(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})
		conn.Read(make([]byte, 256))
		conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	_, err := d.Dial("tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error for general failure")
	}
	if !strings.Contains(err.Error(), "general failure") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDial_HostUnreachable(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})
		conn.Read(make([]byte, 256))
		conn.Write([]byte{0x05, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	_, err := d.Dial("tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error for host unreachable")
	}
	if !strings.Contains(err.Error(), "host unreachable") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDial_UnknownErrorCode(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})
		conn.Read(make([]byte, 256))
		// Unknown error code 0x07
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	_, err := d.Dial("tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error for unknown error code")
	}
	if !strings.Contains(err.Error(), "error code 7") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDial_InvalidAddress(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})
		conn.Read(make([]byte, 256))
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	_, err := d.Dial("tcp", "no-port-specified")
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
	if !strings.Contains(err.Error(), "parse address") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDial_ShortResponse(t *testing.T) {
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})
		conn.Read(make([]byte, 256))
		// Send too-short response
		conn.Write([]byte{0x05})
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	_, err := d.Dial("tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error for short response")
	}
	t.Logf("Short response error: %v", err)
}

func TestDial_EchoServer(t *testing.T) {
	echoData := []byte("hello tor world")
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})
		conn.Read(make([]byte, 256))
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	conn, err := d.Dial("tcp", "echo.onion:7")
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write(echoData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	recv := make([]byte, 1024)
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(recv)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(recv[:n]) != string(echoData) {
		t.Errorf("echo data = %q, want %q", string(recv[:n]), string(echoData))
	}
}

func TestOnionService_Fields(t *testing.T) {
	s := &OnionService{
		OnionAddr: "abcdef1234567890.onion",
		Port:      8080,
	}
	if s.OnionAddr != "abcdef1234567890.onion" {
		t.Errorf("OnionAddr = %q", s.OnionAddr)
	}
	if s.Port != 8080 {
		t.Errorf("Port = %d", s.Port)
	}
}

func TestCreateOnionService_DefaultPort(t *testing.T) {
	_, err := CreateOnionService("127.0.0.1:19999", 0)
	if err == nil {
		t.Error("expected error when Tor not available")
	}
	_, err = CreateOnionService("127.0.0.1:19999", -1)
	if err == nil {
		t.Error("expected error when Tor not available")
	}
}

func TestCreateOnionService_DefaultAddr(t *testing.T) {
	_, err := CreateOnionService("", 8080)
	if err == nil {
		t.Error("expected error when Tor not available")
	}
	t.Logf("Error (expected): %v", err)
}

func TestDial_GreetingWriteError(t *testing.T) {
	// Server accepts then immediately closes
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		conn.Close()
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	_, err := d.Dial("tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error")
	}
	t.Logf("Greeting write error: %v", err)
}

func TestDial_GreetingReadError(t *testing.T) {
	// Server reads greeting then closes without responding
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		conn.Read(make([]byte, 3))
		conn.Close()
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	_, err := d.Dial("tcp", "example.onion:80")
	if err == nil {
		t.Fatal("expected error")
	}
	t.Logf("Greeting read error: %v", err)
}

func TestDial_ConnectWriteError(t *testing.T) {
	// Server does greeting then closes
	ln, addr := startMockSOCKS5(t, func(conn net.Conn) {
		defer conn.Close()
		conn.Read(make([]byte, 3))
		conn.Write([]byte{0x05, 0x00})
		// Close before connect request read
	})
	defer ln.Close()

	d := NewTorDialer(addr)
	// Might succeed or fail depending on timing — just ensure no panic
	_, _ = d.Dial("tcp", "example.onion:80")
}
