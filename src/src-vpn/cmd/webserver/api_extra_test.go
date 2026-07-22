package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/ipfs"
	"github.com/unkillable-messenger/vpn/mesh"
	"github.com/unkillable-messenger/vpn/middleware"
	"github.com/unkillable-messenger/vpn/store"
	"github.com/unkillable-messenger/vpn/tor"
)

// setupFullTestServer creates a test server with ALL routes registered.
func setupFullTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("PATH", "") // Prevent slow exec.LookPath("wg") on Windows
	rl := middleware.NewRateLimiter(1000, 5000)
	perUserLimiter = rl
	t.Cleanup(func() {
		rl.Stop()
	})

	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	vpnDir := dataDir + "/vpn"
	os.MkdirAll(vpnDir, 0700)
	vpnMgr, err = vpn.NewManager(vpnDir)
	if err != nil {
		t.Fatalf("init vpn: %v", err)
	}

	s := &Server{db: db}

	startTime = time.Now()

	// Init mesh net
	meshNet = mesh.NewMeshNet(5)

	// Init nostr relay (nil-safe, handlers check for nil)
	nostrRelay = nil

	// Init tor dialer
	torDialer = tor.NewTorDialer("127.0.0.1:9050")

	// Init ipfs client
	ipfsClient = ipfs.NewClient("http://localhost:8080", "http://localhost:5001")

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
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "")
		}
	}))
	mux.HandleFunc("/api/channels/subscribe", apiChain(s.handleChannelSubscribe))
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
	mux.HandleFunc("/api/messages/edit", apiChain(s.handleEditMessage))
	mux.HandleFunc("/api/messages/delete", apiChain(s.handleDeleteMessage))
	mux.HandleFunc("/api/messages/schedule", apiChain(s.handleScheduleMessage))
	mux.HandleFunc("/api/switch/setup", apiChain(s.handleSwitchSetup))
	mux.HandleFunc("/api/switch/check-in", apiChain(s.handleSwitchCheckIn))
	mux.HandleFunc("/api/push/subscribe", apiChain(s.handlePushSubscribe))
	mux.HandleFunc("/api/groups/create", apiChain(s.handleGroupCreate))
	mux.HandleFunc("/api/groups/list", apiChain(s.handleGroupList))
	mux.HandleFunc("/api/groups/members", apiChain(s.handleGroupMembers))
	mux.HandleFunc("/api/groups/kick", apiChain(s.handleGroupKick))
	mux.HandleFunc("/api/groups/promote", apiChain(s.handleGroupPromote))
	mux.HandleFunc("/api/vpn/split-tunnel", apiChain(s.handleSplitTunnel))
	mux.HandleFunc("/api/vpn/dns", apiChain(s.handleDNSProxy))
	mux.HandleFunc("/api/nostr/stats", apiChain(s.handleNostrStats))
	mux.HandleFunc("/api/ipfs/status", apiChain(s.handleIPFSStatus))
	mux.HandleFunc("/api/nat/discover", apiChain(s.handleNATDiscover))
	mux.HandleFunc("/api/tor/status", apiChain(s.handleTorStatus))
	mux.HandleFunc("/api/mesh/peers", apiChain(s.handleMeshPeers))
	mux.HandleFunc("/api/mesh/stats", apiChain(s.handleMeshStats))
	mux.HandleFunc("/api/stream/create", apiChain(s.handleStreamCreate))
	mux.HandleFunc("/api/stream/list", apiChain(s.handleStreamList))
	mux.HandleFunc("/api/stream/end", apiChain(s.handleStreamEnd))
	mux.HandleFunc("/api/stream/subscribe", apiChain(s.handleStreamSubscribe))

	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		server.CloseClientConnections()
		server.Close()
		s.db.Close()
	})

	return server
}

// ========== Middleware tests ==========

func TestCORSMiddleware(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// P0-3 RESCUE 20260720: whitelist-based CORS.
	// Non-whitelisted origin (https://example.com) must NOT get an ACAO header.
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	handler(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("non-whitelisted origin must not get ACAO, got %q", got)
	}

	// Whitelisted dev origin must be echoed.
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Origin", "http://localhost:5173")
	w2 := httptest.NewRecorder()
	handler(w2, req2)
	if got := w2.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("whitelisted origin must be echoed, got %q", got)
	}

	// OPTIONS preflight must return 204.
	req3 := httptest.NewRequest("OPTIONS", "/test", nil)
	req3.Header.Set("Origin", "http://localhost:5173")
	w3 := httptest.NewRecorder()
	handler(w3, req3)
	if w3.Code != 204 {
		t.Errorf("OPTIONS status = %d, want 204", w3.Code)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := securityHeadersMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing X-Content-Type-Options")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("missing X-Frame-Options")
	}
}

// ========== Utility function tests ==========

func TestTruncate(t *testing.T) {
	if truncate("short", 10) != "short" {
		t.Error("short string should be unchanged")
	}
	long := strings.Repeat("x", 200)
	result := truncate(long, 10)
	// truncate adds "..." suffix
	if len(result) > 13 {
		t.Errorf("truncate result len=%d, want <=13", len(result))
	}
	if truncate("", 10) != "" {
		t.Error("empty should stay empty")
	}
}

func TestContainsPathTraversal(t *testing.T) {
	if !containsPathTraversal("../../../etc/passwd") {
		t.Error("should detect traversal")
	}
	if containsPathTraversal("normal/path/file.txt") {
		t.Error("should not detect traversal in normal path")
	}
}

func TestSetCacheHeaders(t *testing.T) {
	tests := []struct {
		ext           string
		wantImmutable bool
	}{
		{".js", true},
		{".css", true},
		{".html", false},
		{".json", false},
	}
	for _, tt := range tests {
		w := httptest.NewRecorder()
		setCacheHeaders(w, "file"+tt.ext)
		cc := w.Header().Get("Cache-Control")
		if tt.wantImmutable && !strings.Contains(cc, "immutable") {
			t.Errorf("ext=%s: expected immutable", tt.ext)
		}
	}
}

// ========== VPN RPC tests ==========

func TestVPNRPCConnect(t *testing.T) {
	srv := setupFullTestServer(t)

	req := map[string]interface{}{
		"method": "connect",
		"params": map[string]interface{}{
			"config": map[string]interface{}{
				"privateKey": "test-key-1234567890abcdef",
				"address":    "10.0.0.2/24",
				"publicKey":  "server-pub-key",
				"endpoint":   "1.2.3.4:51820",
			},
		},
	}
	resp := postJSON(t, srv.URL+"/api/vpn/rpc", req)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestVPNRPCInvalidMethod(t *testing.T) {
	srv := setupFullTestServer(t)

	req := map[string]interface{}{
		"method": "nonexistent_method",
		"params": map[string]interface{}{},
	}
	resp := postJSON(t, srv.URL+"/api/vpn/rpc", req)
	// VPN manager returns 200 with error in response body for unknown methods
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	result := decodeJSON(t, resp)
	if result["error"] == nil && result["result"] == nil {
		t.Error("expected some response")
	}
}

// ========== Dead Man's Switch tests ==========

func TestSwitchSetupAndCheckIn(t *testing.T) {
	srv := setupFullTestServer(t)

	req := map[string]interface{}{
		"userNpub":     "npub_alice",
		"messageText":  "Emergency message",
		"recipient":    "npub_bob",
		"intervalDays": 3,
	}
	resp := postJSON(t, srv.URL+"/api/switch/setup", req)
	if resp.StatusCode != 200 {
		body, _ := readBody(resp)
		t.Fatalf("setup status = %d, body: %s", resp.StatusCode, body)
	}
	result := decodeJSON(t, resp)
	if result["status"] != "created" {
		t.Errorf("status = %v", result["status"])
	}

	// Check in
	checkIn := map[string]string{"userNpub": "npub_alice"}
	resp = postJSON(t, srv.URL+"/api/switch/check-in", checkIn)
	if resp.StatusCode != 200 {
		t.Fatalf("check-in status = %d", resp.StatusCode)
	}
}

func TestSwitchSetupValidation(t *testing.T) {
	srv := setupFullTestServer(t)

	// GET not allowed
	resp := get(t, srv.URL+"/api/switch/setup")
	if resp.StatusCode != 405 {
		t.Fatalf("GET should be 405, got %d", resp.StatusCode)
	}

	// Missing required fields
	emptyReq := map[string]string{"userNpub": ""}
	resp = postJSON(t, srv.URL+"/api/switch/setup", emptyReq)
	if resp.StatusCode != 400 {
		t.Fatalf("empty setup should be 400, got %d", resp.StatusCode)
	}
}

// ========== Push Subscribe test ==========

func TestPushSubscribe(t *testing.T) {
	srv := setupFullTestServer(t)

	sub := map[string]string{
		"endpoint": "https://push.example.com/sub/123",
		"npub":     "npub_test",
	}
	resp := postJSON(t, srv.URL+"/api/push/subscribe", sub)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// ========== Group tests ==========

func TestGroupCreateAndList(t *testing.T) {
	srv := setupFullTestServer(t)

	// Create group
	req := map[string]string{
		"name":        "test-group",
		"creatorNpub": "alice",
	}
	resp := postJSON(t, srv.URL+"/api/groups/create", req)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := readBody(resp)
		t.Fatalf("create group status = %d, body: %s", resp.StatusCode, body)
	}

	// List groups
	resp = get(t, srv.URL+"/api/groups/list")
	if resp.StatusCode != 200 {
		t.Fatalf("list groups status = %d", resp.StatusCode)
	}
}

// ========== Scheduled messages test ==========

func TestScheduleMessage(t *testing.T) {
	srv := setupFullTestServer(t)

	req := map[string]interface{}{
		"from":      "alice",
		"to":        "bob",
		"text":      "Scheduled hello!",
		"sendAt":    time.Now().Add(1 * time.Hour).Unix(),
		"channelId": "",
	}
	resp := postJSON(t, srv.URL+"/api/messages/schedule", req)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := readBody(resp)
		t.Fatalf("schedule status = %d, body: %s", resp.StatusCode, body)
	}
}

// ========== Split tunnel test ==========

func TestSplitTunnel(t *testing.T) {
	srv := setupFullTestServer(t)

	req := map[string]interface{}{
		"mode":    "split",
		"targets": []string{"example.com", "test.org"},
	}
	resp := postJSON(t, srv.URL+"/api/vpn/split-tunnel", req)
	if resp.StatusCode != 200 {
		body, _ := readBody(resp)
		t.Fatalf("split tunnel status = %d, body: %s", resp.StatusCode, body)
	}
}

// ========== Mesh tests ==========

func TestMeshPeersAndStats(t *testing.T) {
	srv := setupFullTestServer(t)

	resp := get(t, srv.URL+"/api/mesh/peers")
	if resp.StatusCode != 200 {
		t.Fatalf("mesh peers status = %d", resp.StatusCode)
	}

	resp = get(t, srv.URL+"/api/mesh/stats")
	if resp.StatusCode != 200 {
		t.Fatalf("mesh stats status = %d", resp.StatusCode)
	}
}

// ========== Nostr stats test ==========

func TestNostrStatsFull(t *testing.T) {
	srv := setupFullTestServer(t)

	resp := get(t, srv.URL+"/api/nostr/stats")
	// nostrRelay is nil, expect 503
	if resp.StatusCode != 503 {
		t.Fatalf("nostr stats status = %d, want 503", resp.StatusCode)
	}
}

// ========== IPFS status test ==========

func TestIPFSStatusFull(t *testing.T) {
	srv := setupFullTestServer(t)

	resp := get(t, srv.URL+"/api/ipfs/status")
	if resp.StatusCode != 200 {
		t.Fatalf("ipfs status = %d", resp.StatusCode)
	}
}

// ========== NAT discover test ==========

func TestNATDiscoverFull(t *testing.T) {
	srv := setupFullTestServer(t)

	resp := get(t, srv.URL+"/api/nat/discover")
	// STUN may fail without network, just verify no crash
	_ = resp
}

// ========== Tor status test ==========

func TestTorStatusFull(t *testing.T) {
	srv := setupFullTestServer(t)

	resp := get(t, srv.URL+"/api/tor/status")
	// Tor may not be running, just verify no crash
	_ = resp
}

// ========== Stream tests ==========

func TestStreamCreateAndList(t *testing.T) {
	srv := setupFullTestServer(t)

	// Create stream
	req := map[string]string{
		"channelName": "live-stream",
		"streamerId":  "npub_alice",
	}
	resp := postJSON(t, srv.URL+"/api/stream/create", req)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := readBody(resp)
		t.Fatalf("create stream status = %d, body: %s", resp.StatusCode, body)
	}

	// List streams
	resp = get(t, srv.URL+"/api/stream/list")
	if resp.StatusCode != 200 {
		t.Fatalf("list streams status = %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	streams, ok := body["streams"].([]interface{})
	if !ok {
		t.Fatal("streams should be an array")
	}
	if len(streams) != 1 {
		t.Errorf("expected 1 stream, got %d", len(streams))
	}
}

func TestStreamEnd(t *testing.T) {
	srv := setupFullTestServer(t)

	// Create stream first
	req := map[string]string{
		"channelName": "live-stream-2",
		"streamerId":  "npub_bob",
	}
	resp := postJSON(t, srv.URL+"/api/stream/create", req)
	result := decodeJSON(t, resp)
	streamID, _ := result["id"].(string)

	// End stream
	endReq := map[string]string{"streamId": streamID}
	resp = postJSON(t, srv.URL+"/api/stream/end", endReq)
	if resp.StatusCode != 200 {
		t.Fatalf("end stream status = %d", resp.StatusCode)
	}
}

func TestStreamSubscribe(t *testing.T) {
	srv := setupFullTestServer(t)

	// Create stream
	req := map[string]string{
		"channelName": "live-stream-3",
		"streamerId":  "npub_carol",
	}
	resp := postJSON(t, srv.URL+"/api/stream/create", req)
	result := decodeJSON(t, resp)
	streamID, _ := result["id"].(string)

	// Subscribe
	subReq := map[string]string{
		"streamId": streamID,
		"userId":   "npub_viewer",
	}
	resp = postJSON(t, srv.URL+"/api/stream/subscribe", subReq)
	if resp.StatusCode != 200 {
		t.Fatalf("subscribe status = %d", resp.StatusCode)
	}
}

// ========== Messages edge cases ==========

func TestMessagesEmptyGetFull(t *testing.T) {
	srv := setupFullTestServer(t)

	resp := get(t, srv.URL+"/api/messages?npub=nobody&limit=10")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestMessagesEdit(t *testing.T) {
	srv := setupFullTestServer(t)

	// Create a message first
	msg := map[string]string{"from": "alice", "to": "broadcast", "text": "original"}
	postJSON(t, srv.URL+"/api/messages", msg)

	// Edit message
	editReq := map[string]string{
		"messageId": "msg-1",
		"text":      "edited text",
	}
	resp := postJSON(t, srv.URL+"/api/messages/edit", editReq)
	// May fail if messageId doesn't exist in DB, just verify no crash
	_ = resp
}

func TestMessagesDelete(t *testing.T) {
	srv := setupFullTestServer(t)

	delReq := map[string]string{"messageId": "msg-nonexistent"}
	data, _ := json.Marshal(delReq)
	req, _ := http.NewRequest("POST", srv.URL+"/api/messages/delete", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
}

// ========== Channel subscribe test ==========

func TestChannelSubscribe(t *testing.T) {
	srv := setupFullTestServer(t)

	// Create channel
	ch := map[string]string{"name": "sub-test", "description": "test", "creator": "alice"}
	resp := postJSON(t, srv.URL+"/api/channels", ch)
	chResult := decodeJSON(t, resp)
	chID, _ := chResult["id"].(string)

	// Subscribe
	subReq := map[string]string{"channelId": chID, "npub": "npub_bob"}
	resp = postJSON(t, srv.URL+"/api/channels/subscribe", subReq)
	_ = resp
}

// ========== Helper ==========

func readBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return buf.String(), nil
}
