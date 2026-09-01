package main

import (
	"net/http"
	"testing"

	"github.com/unkillable-messenger/vpn/auth"
)

// S1 (2026-09-01): /api/status and /api/files must require JWT.
// These were public before; listing files inventory without auth was a leak.
func TestS1StatusRequiresJWT(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/status without JWT = %d, want 401", resp.StatusCode)
	}
}

func TestS1FilesListRequiresJWT(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/files")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/files without JWT = %d, want 401", resp.StatusCode)
	}
}

func TestS1StatusWithTokenWorks(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	authSvc := auth.NewAuthService("test-secret-setup")
	tok, _, err := authSvc.GenerateTokenPair("user-1", "npub1test")
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/api/status", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/status with JWT = %d, want 200", resp.StatusCode)
	}
}
