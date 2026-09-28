package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

func newTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// seedMessages inserts test messages into the database with recipient-scoped IDs.
func seedMessages(t *testing.T, db *store.Store, count int, recipient string) {
	t.Helper()
	d := db.DB()
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("msg-%s-%d", recipient, i)
		sender := fmt.Sprintf("sender-%d", i%2)
		text := fmt.Sprintf("hello world %d", i)
		_, err := d.Exec(
			"INSERT INTO messages (id, sender, recipient, text, timestamp) VALUES (?, ?, ?, ?, ?)",
			id, sender, recipient, text, int64(1000+i),
		)
		if err != nil {
			t.Fatalf("seed message %d: %v", i, err)
		}
	}
}

// seedChannels inserts test channels into the database.
func seedChannels(t *testing.T, db *store.Store, count int) {
	t.Helper()
	d := db.DB()
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("ch-%d", i)
		name := fmt.Sprintf("channel-%d", i)
		creator := fmt.Sprintf("creator-%d", i)
		_, err := d.Exec(
			"INSERT INTO channels (id, name, creator, created_at) VALUES (?, ?, ?, ?)",
			id, name, creator, int64(2000+i),
		)
		if err != nil {
			t.Fatalf("seed channel %d: %v", i, err)
		}
	}
}

// seedPeers inserts test peers into the database.
func seedPeers(t *testing.T, db *store.Store, count int) {
	t.Helper()
	d := db.DB()
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("peer-%d", i)
		name := fmt.Sprintf("peer-name-%d", i)
		_, err := d.Exec(
			"INSERT INTO peers (id, name) VALUES (?, ?)",
			id, name,
		)
		if err != nil {
			t.Fatalf("seed peer %d: %v", i, err)
		}
	}
}

func TestV2Handler_UnknownResource(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)
	req := httptest.NewRequest("GET", "/api/v2?resource=unknown", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp JSONAPIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == "" {
		t.Error("should have error for unknown resource")
	}
}

func TestV2Handler_Messages(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)
	req := httptest.NewRequest("GET", "/api/v2?resource=messages", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)
	var resp JSONAPIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// No messages yet, but should not error
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

func TestV2Handler_Channels(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)
	req := httptest.NewRequest("GET", "/api/v2?resource=channels", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)
	var resp JSONAPIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

// --- New tests for improved coverage ---

func TestV2Handler_MessagesWithData(t *testing.T) {
	db := newTestDB(t)
	seedMessages(t, db, 3, "general")
	h := NewV2Handler(db)

	req := httptest.NewRequest("GET", "/api/v2?resource=messages", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp JSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be a slice, got %T", resp.Data)
	}
	if len(data) != 3 {
		t.Errorf("expected 3 messages, got %d", len(data))
	}
}

func TestV2Handler_MessagesWithChannelFilter(t *testing.T) {
	db := newTestDB(t)
	seedMessages(t, db, 2, "general")
	seedMessages(t, db, 3, "random")
	h := NewV2Handler(db)

	// Request messages for channel "general" only
	req := httptest.NewRequest("GET", "/api/v2?resource=messages&channel=general", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp JSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be a slice, got %T", resp.Data)
	}
	if len(data) != 2 {
		t.Errorf("expected 2 messages for channel 'general', got %d", len(data))
	}
}

func TestV2Handler_MessagesEmptyChannel(t *testing.T) {
	db := newTestDB(t)
	seedMessages(t, db, 2, "general")
	h := NewV2Handler(db)

	// Request messages for a channel that has no messages
	req := httptest.NewRequest("GET", "/api/v2?resource=messages&channel=nonexistent", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)

	var resp JSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Data should be nil (no results)
	if resp.Data != nil {
		data, ok := resp.Data.([]interface{})
		if ok && len(data) != 0 {
			t.Errorf("expected no messages, got %d", len(data))
		}
	}
}

func TestV2Handler_ChannelsWithData(t *testing.T) {
	db := newTestDB(t)
	seedChannels(t, db, 2)
	h := NewV2Handler(db)

	req := httptest.NewRequest("GET", "/api/v2?resource=channels", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp JSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be a slice, got %T", resp.Data)
	}
	if len(data) != 2 {
		t.Errorf("expected 2 channels, got %d", len(data))
	}

	// Verify channel attributes
	ch := data[0].(map[string]interface{})
	attrs := ch["attributes"].(map[string]interface{})
	if attrs["name"] == "" {
		t.Error("expected channel name to be non-empty")
	}
}

func TestV2Handler_Peers(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)

	req := httptest.NewRequest("GET", "/api/v2?resource=peers", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp JSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
	// Empty DB: data should be nil
	if resp.Data != nil {
		t.Errorf("expected nil data for empty peers, got %v", resp.Data)
	}
}

func TestV2Handler_PeersWithData(t *testing.T) {
	db := newTestDB(t)
	seedPeers(t, db, 3)
	h := NewV2Handler(db)

	req := httptest.NewRequest("GET", "/api/v2?resource=peers", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp JSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be a slice, got %T", resp.Data)
	}
	if len(data) != 3 {
		t.Errorf("expected 3 peers, got %d", len(data))
	}

	// Verify peer attributes
	peer := data[0].(map[string]interface{})
	peerType := peer["type"].(string)
	if peerType != "peer" {
		t.Errorf("expected type 'peer', got %q", peerType)
	}
	attrs := peer["attributes"].(map[string]interface{})
	if attrs["name"] == "" {
		t.Error("expected peer name to be non-empty")
	}
}

func TestV2Handler_MessagesLimit(t *testing.T) {
	// Insert more than 50 messages to test the LIMIT clause
	db := newTestDB(t)
	d := db.DB()
	for i := 0; i < 55; i++ {
		_, err := d.Exec(
			"INSERT INTO messages (id, sender, recipient, text, timestamp) VALUES (?, ?, ?, ?, ?)",
			fmt.Sprintf("msg-limit-%d", i), "sender", "general", fmt.Sprintf("text %d", i), int64(3000+i),
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	h := NewV2Handler(db)
	req := httptest.NewRequest("GET", "/api/v2?resource=messages", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)

	var resp JSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be a slice, got %T", resp.Data)
	}
	if len(data) != 50 {
		t.Errorf("expected 50 messages (LIMIT), got %d", len(data))
	}
}

func TestV2Handler_ResponseContentType(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)

	req := httptest.NewRequest("GET", "/api/v2?resource=channels", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestNewV2Handler(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.db != db {
		t.Error("expected db to be set")
	}
}

func TestV2Handler_EmptyResource(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)

	req := httptest.NewRequest("GET", "/api/v2?resource=", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)

	var resp JSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Empty resource should hit default case
	if resp.Error == "" {
		t.Error("expected error for empty resource")
	}
}
