package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"
)

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
