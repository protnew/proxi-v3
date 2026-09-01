package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/auth"
	"github.com/unkillable-messenger/vpn/middleware"
	"github.com/unkillable-messenger/vpn/storage"
	"github.com/unkillable-messenger/vpn/store"
)

// setupTestServer creates a fully initialized test server with in-memory DB.
func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Create temp dirs
	dataDir := t.TempDir()
	uploadDir := filepath.Join(dataDir, "uploads")
	os.MkdirAll(uploadDir, 0700)

	// Override getDataDir and uploadDir via env
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("PATH", "") // Prevent slow exec.LookPath("wg") on Windows

	// Initialize storage provider for tests
	storageProvider, _ = storage.NewLocalStore(dataDir)

	rl := middleware.NewRateLimiter(1000, 5000)
	perUserLimiter = rl
	t.Cleanup(func() {
		rl.Stop()
	})

	// Initialize store
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Initialize VPN manager
	vpnDir := filepath.Join(dataDir, "vpn")
	os.MkdirAll(vpnDir, 0700)
	vpnMgr, err = vpn.NewManager(vpnDir)
	if err != nil {
		t.Fatalf("init vpn: %v", err)
	}

	s := &Server{db: db}

	startTime = time.Now()

	// Build mux
	mux := http.NewServeMux()

	apiChain := func(h http.HandlerFunc) http.HandlerFunc {
		return securityHeadersMiddleware(corsMiddleware(rateLimitMiddleware(h)))
	}
	// S1 (2026-09-01): mirror prod chains — status/files behind JWT like registerRoutes does
	testAuth := auth.NewAuthService("test-secret-setup")
	protected := func(h http.HandlerFunc) http.HandlerFunc {
		return securityHeadersMiddleware(corsMiddleware(rateLimitMiddleware(authMiddleware(testAuth, h))))
	}

	mux.HandleFunc("/api/health", apiChain(s.handleHealth))
	mux.HandleFunc("/api/status", protected(s.handleStatus))
	mux.HandleFunc("/api/messages", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			s.handleMessagesGet(w, r)
		case "POST":
			s.handleMessagesPost(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
		}
	}))
	mux.HandleFunc("/api/channels", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			s.handleChannelsGet(w, r)
		case "POST":
			s.handleChannelsPost(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
		}
	}))
	mux.HandleFunc("/api/vpn/rpc", apiChain(s.handleVpnRPC))
	mux.HandleFunc("/api/federation/peer", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			s.handleFederationPeerList(w, r)
		case "POST":
			s.handleFederationPeerAdd(w, r)
		case "DELETE":
			s.handleFederationPeerRemove(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "")
		}
	}))
	mux.HandleFunc("/api/identity", apiChain(s.handleIdentityGet))
	mux.HandleFunc("/api/files/upload", apiChain(s.handleFileUpload))
	mux.HandleFunc("/api/files/", protected(s.handleFileGet))
	mux.HandleFunc("/api/reactions", apiChain(s.handleReactions))
	mux.HandleFunc("/api/read-receipts", apiChain(s.handleReadReceipts))
	mux.HandleFunc("/api/profiles", apiChain(s.handleProfiles))
	mux.HandleFunc("/api/search", apiChain(s.handleSearch))

	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		server.CloseClientConnections()
		server.Close()
		s.db.Close()
	})

	return server
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// getAuth issues a GET with a valid Bearer token signed by the harness secret.
func getAuth(t *testing.T, url string) *http.Response {
	t.Helper()
	authSvc := auth.NewAuthService("test-secret-setup")
	tok, _, err := authSvc.GenerateTokenPair("user-1", "npub1test")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET(auth) %s: %v", url, err)
	}
	return resp
}

func postJSON(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return result
}

// ========== Tests ==========

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)
	resp := get(t, srv.URL+"/api/health")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["status"] != "ok" {
		t.Errorf("health status = %v, want ok", body["status"])
	}
	if body["version"] != "0.1.0" {
		t.Errorf("version = %v, want 0.1.0", body["version"])
	}
}

func TestStatusEndpoint(t *testing.T) {
	srv := setupTestServer(t)
	// S1: without JWT the status endpoint must refuse
	resp := get(t, srv.URL+"/api/status")
	if resp.StatusCode != 401 {
		t.Fatalf("status without JWT = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()
	resp = getAuth(t, srv.URL+"/api/status")
	if resp.StatusCode != 200 {
		t.Fatalf("status with JWT = %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["status"] != "running" {
		t.Errorf("status = %v, want running", body["status"])
	}
	vpnInfo, ok := body["vpnInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("vpnInfo should be a map")
	}
	if vpnInfo["state"] != "disconnected" {
		t.Errorf("vpn state = %v, want disconnected", vpnInfo["state"])
	}
}

func TestIdentityCreateAndGet(t *testing.T) {
	srv := setupTestServer(t)

	// First call creates identity
	resp := get(t, srv.URL+"/api/identity")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	npub, _ := body["npub"].(string)
	if npub == "" {
		t.Error("npub should not be empty")
	}
	if body["isNew"] != true {
		t.Errorf("isNew should be true on first call, got %v", body["isNew"])
	}

	// Second call returns existing identity
	resp = get(t, srv.URL+"/api/identity")
	body = decodeJSON(t, resp)
	if body["isNew"] == true {
		t.Error("isNew should be false on second call")
	}
	if body["npub"] != npub {
		t.Error("npub should be the same on second call")
	}
}

func TestMessagesPostAndGet(t *testing.T) {
	srv := setupTestServer(t)

	// POST a message
	msg := map[string]string{
		"from": "alice",
		"to":   "broadcast",
		"text": "Hello from test!",
	}
	resp := postJSON(t, srv.URL+"/api/messages", msg)
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("POST messages status = %d, body: %s", resp.StatusCode, body)
	}
	created := decodeJSON(t, resp)
	if created["text"] != "Hello from test!" {
		t.Errorf("text = %v", created["text"])
	}

	// GET messages
	resp = get(t, srv.URL+"/api/messages?npub=alice&limit=100")
	if resp.StatusCode != 200 {
		t.Fatalf("GET messages status = %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	msgs, ok := body["messages"].([]interface{})
	if !ok {
		t.Fatal("messages should be an array")
	}
	if len(msgs) < 1 {
		t.Error("expected at least 1 message")
	}
}

func TestChannelsPostAndGet(t *testing.T) {
	srv := setupTestServer(t)

	ch := map[string]string{
		"name":        "test-channel",
		"description": "A test channel",
		"creator":     "alice",
	}
	resp := postJSON(t, srv.URL+"/api/channels", ch)
	if resp.StatusCode != 201 {
		t.Fatalf("POST channels status = %d", resp.StatusCode)
	}
	created := decodeJSON(t, resp)
	if created["name"] != "test-channel" {
		t.Errorf("name = %v", created["name"])
	}

	// GET channels
	resp = get(t, srv.URL+"/api/channels")
	body := decodeJSON(t, resp)
	channels, ok := body["channels"].([]interface{})
	if !ok {
		t.Fatal("channels should be an array")
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
}

func TestVPNRPCGetStatus(t *testing.T) {
	srv := setupTestServer(t)

	req := map[string]interface{}{
		"method": "get_status",
		"params": map[string]interface{}{},
	}
	resp := postJSON(t, srv.URL+"/api/vpn/rpc", req)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	result, ok := body["result"].(map[string]interface{})
	if !ok {
		t.Fatal("should have result field")
	}
	if result["state"] != "disconnected" {
		t.Errorf("vpn state = %v", result["state"])
	}
}
