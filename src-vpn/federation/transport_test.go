package federation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn/nostr"
)

// ==================== Transport Tests ====================

func TestSendEvent(t *testing.T) {
	received := make(chan nostr.Event, 1)

	// Mock server that accepts events
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/federation/event" {
			t.Errorf("expected path /api/federation/event, got %s", r.URL.Path)
		}

		var event nostr.Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("decode event: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		received <- event
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := NewHTTPTransport()
	transport.AddPeer("peer1", server.URL)

	testEvent := nostr.Event{
		ID:        "testevent123",
		PubKey:    "testpubkey",
		CreatedAt: time.Now().Unix(),
		Kind:      1,
		Content:   "Hello, federation!",
	}

	err := transport.SendEvent("peer1", testEvent)
	if err != nil {
		t.Fatalf("SendEvent returned error: %v", err)
	}

	select {
	case evt := <-received:
		if evt.ID != testEvent.ID {
			t.Errorf("expected event ID %s, got %s", testEvent.ID, evt.ID)
		}
		if evt.Content != testEvent.Content {
			t.Errorf("expected content %s, got %s", testEvent.Content, evt.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event to be received")
	}
}

func TestFetchEvents(t *testing.T) {
	testEvents := []nostr.Event{
		{ID: "evt1", PubKey: "pk1", CreatedAt: 1000, Kind: 1, Content: "msg1"},
		{ID: "evt2", PubKey: "pk2", CreatedAt: 2000, Kind: 1, Content: "msg2"},
		{ID: "evt3", PubKey: "pk3", CreatedAt: 3000, Kind: 4, Content: "dm"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/federation/events" {
			t.Errorf("expected path /api/federation/events, got %s", r.URL.Path)
		}

		since := r.URL.Query().Get("since")
		if since == "" {
			t.Error("expected 'since' query parameter")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testEvents)
	}))
	defer server.Close()

	transport := NewHTTPTransport()
	transport.AddPeer("peer1", server.URL)

	events, err := transport.FetchEvents("peer1", 500)
	if err != nil {
		t.Fatalf("FetchEvents returned error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].ID != "evt1" {
		t.Errorf("expected first event ID 'evt1', got '%s'", events[0].ID)
	}
	if events[2].Content != "dm" {
		t.Errorf("expected third event content 'dm', got '%s'", events[2].Content)
	}
}

func TestSyncAll(t *testing.T) {
	testEvents := []nostr.Event{
		{ID: "sync1", PubKey: "pk1", CreatedAt: time.Now().Unix(), Kind: 1, Content: "synced msg"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testEvents)
	}))
	defer server.Close()

	transport := NewHTTPTransport()
	transport.AddPeer("peer1", server.URL)
	transport.AddPeer("peer2", server.URL) // same server, two peers

	relay := NewFederatedRelay(nostr.NewRelay(1000, nil))

	err := transport.SyncAll(context.Background(), relay)
	if err != nil {
		t.Fatalf("SyncAll returned error: %v", err)
	}
}

func TestSendEvent_PeerDown(t *testing.T) {
	transport := NewHTTPTransport()
	// Use a port that nothing is listening on
	transport.AddPeer("dead-peer", "http://127.0.0.1:1")

	// Override retry settings for faster test
	transport.client.Timeout = 100 * time.Millisecond

	testEvent := nostr.Event{
		ID:        "testevent",
		PubKey:    "pk",
		CreatedAt: time.Now().Unix(),
		Kind:      1,
		Content:   "test",
	}

	err := transport.SendEvent("dead-peer", testEvent)
	if err == nil {
		t.Error("expected error when sending to unreachable peer")
	}
}

func TestSendEvent_PeerNotFound(t *testing.T) {
	transport := NewHTTPTransport()

	testEvent := nostr.Event{ID: "test", PubKey: "pk", Kind: 1, Content: "test"}

	err := transport.SendEvent("nonexistent", testEvent)
	if err == nil {
		t.Error("expected error for unknown peer")
	}
}

func TestFetchEvents_PeerNotFound(t *testing.T) {
	transport := NewHTTPTransport()

	_, err := transport.FetchEvents("nonexistent", 0)
	if err == nil {
		t.Error("expected error for unknown peer")
	}
}

func TestHTTPTransport_Concurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]nostr.Event{})
		}
	}))
	defer server.Close()

	transport := NewHTTPTransport()
	transport.AddPeer("peer1", server.URL)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			evt := nostr.Event{ID: "evt", PubKey: "pk", Kind: 1, Content: "msg"}
			_ = transport.SendEvent("peer1", evt)
		}(i)
	}
	wg.Wait()
}
