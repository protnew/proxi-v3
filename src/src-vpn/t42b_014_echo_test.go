package vpn

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// T42B-014: tun2socks and the echo autotest share T42BConfig.socksPort (10808).
func TestT42B014_CanonPortEcho(t *testing.T) {
	if T42BSocksPort != 10808 {
		t.Fatalf("Go port %d != Android T42BConfig.socksPort 10808", T42BSocksPort)
	}
	listen := fmt.Sprintf("%s:%d", T42BSocksHost, T42BSocksPort)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("echo-10808"))
	}))
	defer backend.Close()

	probe, err := net.Listen("tcp", listen)
	if err != nil {
		t.Fatalf("canon port %s busy: %v", listen, err)
	}
	_ = probe.Close()

	srv := NewSOCKS5Server(listen, "")
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	target := backend.Listener.Addr().String()
	conn, err := dialViaSOCKS5(listen, target, 5*time.Second)
	if err != nil {
		t.Fatalf("echo via %s: %v", listen, err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("GET / HTTP/1.0\r\nHost: t\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	buf, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(buf), "echo-10808") {
		t.Fatalf("body=%q", buf)
	}
}
