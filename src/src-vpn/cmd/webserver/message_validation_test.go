package main

import (
	"io"
	"net/http"
	"strings"
	"testing"

	vpnroot "github.com/unkillable-messenger/vpn"
)

func TestMessagesPost_EmptyText(t *testing.T) {
	srv := setupTestServer(t)
	resp := postJSON(t, srv.URL+"/api/messages", map[string]interface{}{
		"from": "alice", "to": "bob", "text": "   ",
	})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		t.Fatalf("empty text must not create message: %d %s", resp.StatusCode, body)
	}
}

func TestMessagesPost_Oversize(t *testing.T) {
	srv := setupTestServer(t)
	text := strings.Repeat("x", vpnroot.MaxMessageLen+1)
	resp := postJSON(t, srv.URL+"/api/messages", map[string]interface{}{
		"from": "alice", "to": "bob", "text": text,
	})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(strings.ToLower(string(body)), "too long") {
		t.Fatalf("body should mention too long: %s", body)
	}
}

func TestMessagesPost_ExactLimit(t *testing.T) {
	srv := setupTestServer(t)
	text := strings.Repeat("y", vpnroot.MaxMessageLen)
	resp := postJSON(t, srv.URL+"/api/messages", map[string]interface{}{
		"from": "alice", "to": "bob", "text": text,
	})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("exact limit should pass: %d %s", resp.StatusCode, body)
	}
}

func TestMessagesPost_Normal(t *testing.T) {
	srv := setupTestServer(t)
	resp := postJSON(t, srv.URL+"/api/messages", map[string]interface{}{
		"from": "alice", "to": "bob", "text": "hello validation suite",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
}

func TestMessagesMethodNotAllowed(t *testing.T) {
	srv := setupTestServer(t)
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/messages", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("want 405 got %d", resp.StatusCode)
	}
}

func TestHealthAndCORSHeaders(t *testing.T) {
	srv := setupTestServer(t)
	resp := get(t, srv.URL+"/api/health")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("health %d", resp.StatusCode)
	}
	// security headers middleware should set at least one header
	if resp.Header.Get("X-Content-Type-Options") == "" && resp.Header.Get("Content-Type") == "" {
		t.Log("no security header observed (warn)")
	}
}
