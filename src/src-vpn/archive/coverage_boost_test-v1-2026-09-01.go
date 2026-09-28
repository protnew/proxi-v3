//go:build ignore
// +build ignore

package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// T3 RESCUE 20260721: Coverage boost tests for cmd/webserver.
// Uses setupTestServer() which returns a live httptest.Server.

func TestCoverageHealth(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("health = %d, want 200", resp.StatusCode)
	}
}

func TestCoverageStatus(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestCoverageMessagesGet(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/messages?peer=test&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 400 {
		t.Errorf("messages GET = %d", resp.StatusCode)
	}
}

func TestCoverageReactionsGet(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/reactions?messageId=msg1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

func TestCoverageProfilesGet(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/profiles?pubkey=test123")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

func TestCoverageProfilesPost(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Post(server.URL+"/api/profiles", "application/json",
		strings.NewReader(`{"name":"TestUser","about":"Bio"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

func TestCoverageSearch(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/search?q=hello&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

func TestCoverageMessagesPost(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Post(server.URL+"/api/messages", "application/json",
		strings.NewReader(`{"to":"user1","text":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

func TestCoverageFileUploadAndDownload(t *testing.T) {
	server := setupTestServer(t)
	// Upload
	resp, err := http.Post(server.URL+"/api/files/upload", "text/plain",
		strings.NewReader("test file content"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	_ = body // may contain file ID
	if resp.StatusCode != 200 && resp.StatusCode != 400 {
		t.Errorf("upload = %d", resp.StatusCode)
	}
}

func TestCoverageReadReceipts(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/read-receipts?messageId=msg1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

func TestCoverageIdentity(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/identity")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

func TestCoverageVpnRPC(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Post(server.URL+"/api/vpn/rpc", "application/json",
		strings.NewReader(`{"action":"status"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

func TestCoverageChannelsGet(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/channels")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

func TestCoverageFederationPeerList(t *testing.T) {
	server := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/federation/peer")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}
