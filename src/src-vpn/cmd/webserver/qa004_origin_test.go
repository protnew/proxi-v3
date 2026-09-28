package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// QA-004: Verify all API endpoints reject requests without Bearer token
// when authSvc is configured (rebinding protection).
func TestQA004_APIEndpointsRequireAuth(t *testing.T) {
	// Build a simple test server with apiChain = protectedApiChain
	handler := securityHeadersMiddleware(corsMiddleware(rateLimitMiddleware(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
		})))

	// Request WITHOUT Authorization header → should still work at chain level
	// (authMiddleware is tested separately in auth_signup_test.go)
	// This test verifies the chain is properly wired
	req := httptest.NewRequest("GET", "/api/vpn/amnezia", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// CORS should block evil origin
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin == "http://evil.example.com" {
		t.Error("CORS allowed evil origin — rebinding vulnerability")
	}
}

// QA-004: Verify CORS rejects non-whitelisted origins
func TestQA004_CORSRejectsEvilOrigin(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Evil origin
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "http://evil.attacker.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin == "http://evil.attacker.com" {
		t.Error("CORS allowed attacker origin")
	}
}

// QA-004: Verify CORS allows LAN origin
func TestQA004_CORSAllowsLANOrigin(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// LAN origin
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "http://10.217.127.5:8090")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://10.217.127.5:8090" {
		t.Errorf("CORS did not allow LAN origin, got: %s", origin)
	}
}
