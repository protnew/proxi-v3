package vpn

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSOCKS5RealTraffic(t *testing.T) {
	// backend HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello-via-socks"))
	}))
	defer ts.Close()

	// pick free port for socks
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	s := NewSOCKS5Server(addr, "")
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	// dial target through socks
	hostport := ts.Listener.Addr().String()
	conn, err := dialViaSOCKS5(addr, hostport, 5*time.Second)
	if err != nil {
		t.Fatalf("dialViaSOCKS5: %v", err)
	}
	defer conn.Close()

	req := "GET / HTTP/1.0\r\nHost: test\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	buf, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(buf), "hello-via-socks") {
		t.Fatalf("unexpected body: %q", string(buf))
	}
	in, out, _ := s.Stats()
	if in == 0 && out == 0 {
		t.Fatalf("expected byte counters > 0, got in=%d out=%d", in, out)
	}
}

func TestManagerStartRealTunnel(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	if err := m.StartRealTunnel(addr, ""); err != nil {
		t.Fatal(err)
	}
	st := m.GetStatus()
	if st.State != StateConnected {
		t.Fatalf("state=%s", st.State)
	}
	if !st.RealTraffic {
		t.Fatal("expected realTraffic")
	}
	if st.SocksAddr != addr {
		t.Fatalf("socks=%s want %s", st.SocksAddr, addr)
	}
	if err := m.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if m.GetStatus().State != StateDisconnected {
		t.Fatal("not disconnected")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
