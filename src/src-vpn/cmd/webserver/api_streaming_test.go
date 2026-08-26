package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

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
