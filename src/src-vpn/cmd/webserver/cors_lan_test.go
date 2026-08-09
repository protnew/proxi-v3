package main

import (
	"net/http/httptest"
	"testing"
)

// SEC-005: CORS whitelist + LAN origins regression
func TestIsPrivateLANOrigin(t *testing.T) {
	cases := map[string]bool{
		// localhost variants
		"http://127.0.0.1:8090":   true,
		"http://localhost:8090":   true,
		"http://localhost:5173":   true,
		"http://127.0.0.1:5173":   true,
		// private Class C
		"http://192.168.1.5:8090": true,
		"http://192.168.0.1:8090": true,
		"http://192.168.100.2:3000": true,
		// private Class A
		"http://10.0.0.2:8090":    true,
		"http://10.217.127.5:8090": true,
		"http://10.255.255.255:8090": true,
		// private Class B (172.16-31)
		"http://172.16.0.1:8090":  true,
		"http://172.31.255.255:8090": true,
		// link-local
		"http://169.254.1.1:8090": true,
		// public — must reject
		"http://8.8.8.8:8090":     false,
		"https://evil.com":        false,
		"http://100.64.0.1:8090":  false, // CGNAT — not LAN
		"http://172.32.0.1:8090":  false, // just outside 172.16/12
		"http://192.169.1.1:8090": false, // public, not 192.168
		"http://11.0.0.1:8090":    false, // public, not 10/8
		// malformed
		"":                        false,
		"javascript:alert(1)":     false,
		"data:text/html,evil":     false,
		"file:///etc/passwd":      false,
	}
	for o, want := range cases {
		got := isPrivateLANOrigin(o)
		if got != want {
			t.Errorf("isPrivateLANOrigin(%q) = %v, want %v", o, got, want)
		}
	}
}

// SEC-005: Verify CORS headers are applied for LAN origins
func TestCORSMiddlewareLANHeaders(t *testing.T) {
	origins := []string{
		"http://192.168.1.100:8090",
		"http://10.0.0.5:8090",
		"http://172.16.0.1:8090",
	}
	for _, origin := range origins {
		req := httptest.NewRequest("OPTIONS", "/api/health", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()

		ok := applyCORSHeaders(w, req, true)

		if !ok {
			t.Errorf("Origin %s: applyCORSHeaders returned false", origin)
		}
		acao := w.Header().Get("Access-Control-Allow-Origin")
		if acao != origin {
			t.Errorf("Origin %s: ACAC header = %q, want %q", origin, acao, origin)
		}
	}
}

// SEC-005: Verify CORS blocks public origins
func TestCORSMiddlewareBlocksPublic(t *testing.T) {
	publicOrigins := []string{
		"https://evil.com",
		"http://8.8.8.8:8090",
		"http://100.64.0.1:8090",
	}
	for _, origin := range publicOrigins {
		req := httptest.NewRequest("OPTIONS", "/api/health", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()

		applyCORSHeaders(w, req, true)

		acao := w.Header().Get("Access-Control-Allow-Origin")
		if acao != "" {
			t.Errorf("Public origin %s: ACAC header should be empty, got %q", origin, acao)
		}
	}
}
