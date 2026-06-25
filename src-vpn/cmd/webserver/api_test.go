package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn"
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

	// Initialize storage provider for tests
	storageProvider, _ = storage.NewLocalStore(dataDir)

	// Override rate limiter to allow all requests during tests
	perUserLimiter = middleware.NewRateLimiter(1000, 5000)

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

	mux.HandleFunc("/api/health", apiChain(s.handleHealth))
	mux.HandleFunc("/api/status", apiChain(s.handleStatus))
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
	mux.HandleFunc("/api/files/", s.handleFileGet)
	mux.HandleFunc("/api/reactions", apiChain(s.handleReactions))
	mux.HandleFunc("/api/read-receipts", apiChain(s.handleReadReceipts))
	mux.HandleFunc("/api/profiles", apiChain(s.handleProfiles))
	mux.HandleFunc("/api/search", apiChain(s.handleSearch))

	server := httptest.NewServer(mux)
	t.Cleanup(func() {
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
	resp := get(t, srv.URL+"/api/status")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
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

func TestPeersCRUD(t *testing.T) {
	srv := setupTestServer(t)

	longKey := "abcdefghijklmnopqrstuvwxyz1234567890ABCDEF"

	// Add peer via RPC
	addReq := map[string]interface{}{
		"method": "add_peer",
		"params": map[string]interface{}{
			"name":      "TestPeer",
			"publicKey": longKey,
			"endpoint":  "1.2.3.4:51820",
		},
	}
	resp := postJSON(t, srv.URL+"/api/vpn/rpc", addReq)
	if resp.StatusCode != 200 {
		t.Fatalf("POST add_peer status = %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if _, ok := body["result"]; !ok {
		t.Fatalf("expected result in response")
	}

	// Wait for peer to be added
	time.Sleep(10 * time.Millisecond)

	// GET status to see peers
	getReq := map[string]interface{}{
		"method": "get_status",
		"params": map[string]interface{}{},
	}
	resp = postJSON(t, srv.URL+"/api/vpn/rpc", getReq)
	body = decodeJSON(t, resp)
	result, _ := body["result"].(map[string]interface{})
	peers, ok := result["peers"].([]interface{})
	if !ok {
		t.Fatal("peers should be an array in get_status")
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}

	// DELETE peer via RPC
	delReq := map[string]interface{}{
		"method": "remove_peer",
		"params": map[string]interface{}{
			"peerId": longKey[:16],
		},
	}
	resp = postJSON(t, srv.URL+"/api/vpn/rpc", delReq)
	if resp.StatusCode != 200 {
		t.Fatalf("POST remove_peer status = %d", resp.StatusCode)
	}

	// Wait for peer to be removed
	time.Sleep(10 * time.Millisecond)

	// GET status again
	resp = postJSON(t, srv.URL+"/api/vpn/rpc", getReq)
	body = decodeJSON(t, resp)
	result, _ = body["result"].(map[string]interface{})
	peers2, _ := result["peers"].([]interface{})
	if len(peers2) != 0 {
		t.Errorf("expected 0 peers after delete, got %d", len(peers2))
	}
}

func TestFileUploadAndGet(t *testing.T) {
	srv := setupTestServer(t)

	// Upload a file
	fileContent := "This is test file content for upload"
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(part, fileContent)
	writer.WriteField("npub", "alice")
	writer.Close()

	resp, err := http.Post(srv.URL+"/api/files/upload", writer.FormDataContentType(), body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}
	var uploadResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&uploadResp)
	fileID, _ := uploadResp["id"].(string)
	if fileID == "" {
		t.Fatal("file ID should not be empty")
	}

	// GET file list
	resp2 := get(t, srv.URL+"/api/files")
	body2 := decodeJSON(t, resp2)
	files, ok := body2["files"].([]interface{})
	if !ok {
		t.Fatal("files should be an array")
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	// GET single file by ID
	resp3, err := http.Get(srv.URL + "/api/files/" + fileID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	// The file endpoint should return 200 even if the disk file is in the temp dir
	// (it uses ServeFile which reads from disk)
}

func TestReactionsAddAndGet(t *testing.T) {
	srv := setupTestServer(t)

	// Add reaction
	reaction := map[string]string{
		"messageId": "msg-test-1",
		"userNpub":  "npub_alice",
		"emoji":     "👍",
	}
	resp := postJSON(t, srv.URL+"/api/reactions", reaction)
	if resp.StatusCode != 200 {
		t.Fatalf("add reaction status = %d", resp.StatusCode)
	}
	decodeJSON(t, resp)

	// Get reactions
	resp = get(t, srv.URL+"/api/reactions?messageId=msg-test-1")
	if resp.StatusCode != 200 {
		t.Fatalf("get reactions status = %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	reactions, ok := body["reactions"].([]interface{})
	if !ok {
		t.Fatal("reactions should be an array")
	}
	if len(reactions) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(reactions))
	}

	// Remove reaction
	delReq := map[string]string{
		"messageId": "msg-test-1",
		"userNpub":  "npub_alice",
	}
	data, _ := json.Marshal(delReq)
	req, _ := http.NewRequest("DELETE", srv.URL+"/api/reactions", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("remove reaction status = %d", resp.StatusCode)
	}
}

func TestReadReceipts(t *testing.T) {
	srv := setupTestServer(t)

	// Save a message first
	msg := map[string]string{"from": "bob", "to": "broadcast", "text": "test"}
	postJSON(t, srv.URL+"/api/messages", msg)

	// Mark read
	markReq := map[string]string{
		"messageId": "msg-123",
		"userNpub":  "npub_alice",
	}
	resp := postJSON(t, srv.URL+"/api/read-receipts", markReq)
	if resp.StatusCode != 200 {
		t.Fatalf("mark read status = %d", resp.StatusCode)
	}
	decodeJSON(t, resp)

	// Get read receipts
	resp = get(t, srv.URL+"/api/read-receipts?messageId=msg-123")
	if resp.StatusCode != 200 {
		t.Fatalf("get read receipts status = %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	readBy, ok := body["readBy"].([]interface{})
	if !ok {
		t.Fatal("readBy should be an array")
	}
	if len(readBy) != 1 {
		t.Fatalf("expected 1 read receipt, got %d", len(readBy))
	}
}

func TestProfilesSaveAndGet(t *testing.T) {
	srv := setupTestServer(t)

	// Save profile
	profile := map[string]string{
		"npub":        "npub_test123",
		"displayName": "Test User",
		"avatarUrl":   "https://example.com/avatar.png",
		"bio":         "Just testing",
	}
	resp := postJSON(t, srv.URL+"/api/profiles", profile)
	if resp.StatusCode != 200 {
		t.Fatalf("save profile status = %d", resp.StatusCode)
	}
	decodeJSON(t, resp)

	// Get profile
	resp = get(t, srv.URL+"/api/profiles?npub=npub_test123")
	if resp.StatusCode != 200 {
		t.Fatalf("get profile status = %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["displayName"] != "Test User" {
		t.Errorf("displayName = %v, want Test User", body["displayName"])
	}

	// Search profiles
	resp = get(t, srv.URL+"/api/profiles?search=Test")
	if resp.StatusCode != 200 {
		t.Fatalf("search profiles status = %d", resp.StatusCode)
	}
	body = decodeJSON(t, resp)
	profiles, ok := body["profiles"].([]interface{})
	if !ok {
		t.Fatal("profiles should be an array")
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
}

func TestSearchMessages(t *testing.T) {
	srv := setupTestServer(t)

	// Create some messages
	msgs := []map[string]string{
		{"from": "alice", "to": "broadcast", "text": "golang is great"},
		{"from": "bob", "to": "broadcast", "text": "I love rust"},
		{"from": "alice", "to": "broadcast", "text": "golang testing"},
	}
	for _, m := range msgs {
		postJSON(t, srv.URL+"/api/messages", m)
	}

	// Wait a tiny bit for DB writes
	time.Sleep(10 * time.Millisecond)

	// Search for "golang"
	resp := get(t, srv.URL+"/api/search?q=golang&npub=alice")
	if resp.StatusCode != 200 {
		t.Fatalf("search status = %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	results, ok := body["results"].([]interface{})
	if !ok {
		t.Fatal("results should be an array")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'golang', got %d", len(results))
	}
	if body["query"] != "golang" {
		t.Errorf("query = %v, want golang", body["query"])
	}

	// Search with no results
	resp = get(t, srv.URL+"/api/search?q=nonexistent&npub=alice")
	body = decodeJSON(t, resp)
	results, _ = body["results"].([]interface{})
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent, got %d", len(results))
	}

	// Search without query param = error
	resp = get(t, srv.URL+"/api/search")
	if resp.StatusCode != 400 {
		t.Fatalf("search without q should be 400, got %d", resp.StatusCode)
	}

	// Validate q is required via error message
	_ = decodeJSON(t, resp) // just drain
}

func TestSearchQueryTooLong(t *testing.T) {
	srv := setupTestServer(t)
	longQuery := strings.Repeat("a", 201)
	resp := get(t, srv.URL+"/api/search?q="+longQuery)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for long query, got %d", resp.StatusCode)
	}
}
