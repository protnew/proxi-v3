package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/tun/netstack"
)

func TestNetstack_RoundTrip(t *testing.T) {
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "pong-from-httptest")
	}))
	defer ht.Close()

	dev, ns, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("10.7.0.2")},
		[]netip.Addr{netip.MustParseAddr("10.7.0.1")},
		1280,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()

	ln, err := ns.ListenTCPAddrPort(netip.MustParseAddrPort("10.7.0.2:18080"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	errCh := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer c.Close()
		_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 1024)
		_, _ = c.Read(buf)
		resp, err := http.Get(ht.URL)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		_, err = c.Write(body)
		errCh <- err
	}()

	conn, err := ns.DialTCPAddrPort(netip.MustParseAddrPort("10.7.0.2:18080"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte("GET / HTTP/1.0\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if string(buf[:n]) != "pong-from-httptest" {
		t.Fatalf("body=%q", buf[:n])
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("proxy side timeout")
	}
	_ = net.IPv4len
}
