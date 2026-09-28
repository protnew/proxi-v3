package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestF10_EvilHostRejected(t *testing.T) {
	h := hostGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest(http.MethodGet, "http://evil.com/api/network/lan", nil)
	req.Host = "evil.com"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code < 400 {
		t.Fatalf("Host evil.com status %d", rec.Code)
	}
	req2 := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/health", nil)
	req2.Host = "127.0.0.1:8090"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("loopback host status %d", rec2.Code)
	}
}

func TestF11_BusyPortRecovers(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:8090")
	if err != nil {
		t.Skip("8090 not free for the busy-port fixture")
	}
	defer busy.Close()
	ln, err := listenLoopback("8090")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if ln.Addr().String() == "127.0.0.1:8090" {
		t.Fatal("expected recover off 8090")
	}
	host, _, _ := net.SplitHostPort(ln.Addr().String())
	if host != "127.0.0.1" {
		t.Fatalf("not loopback: %s", ln.Addr())
	}
}

func TestF11_JWTUniquePerCall(t *testing.T) {
	t.Setenv("JWT_SECRET_PIN", "")
	t.Setenv("JWT_SECRET", "static-from-dotenv-must-not-be-used")
	a, err := perBootJWTSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, err := perBootJWTSecret()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a == "static-from-dotenv-must-not-be-used" {
		t.Fatalf("secret not per-boot: %q %q", a, b)
	}
}

func TestF9_TauriOriginAllowed(t *testing.T) {
	if !wsOriginAllowed("http://tauri.localhost") || !wsOriginAllowed("tauri://localhost") {
		t.Fatal("tauri origins must pass WS whitelist")
	}
}
