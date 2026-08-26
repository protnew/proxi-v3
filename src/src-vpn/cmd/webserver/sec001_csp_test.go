package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSEC001_CSPHeaderPresent(t *testing.T) {
	h := securityHeadersMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("CSP header missing")
	}
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP missing default-src self: %s", csp)
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP must restrict scripts to self (no unsafe-eval for nsec theft): %s", csp)
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Error("script-src must NOT allow unsafe-inline")
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Error("frame-ancestors none required")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options DENY required")
	}
}
