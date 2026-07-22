package federation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/unkillable-messenger/vpn/nostr"
)

func TestNewFederatedRelay(t *testing.T) {
	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)
	if fr == nil {
		t.Fatal("NewFederatedRelay returned nil")
	}
	if len(fr.GetPeers()) != 0 {
		t.Fatalf("expected 0 peers, got %d", len(fr.GetPeers()))
	}
}

func TestAddPeer(t *testing.T) {
	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)

	if err := fr.AddPeer("http://peer1.example.com"); err != nil {
		t.Fatalf("AddPeer failed: %v", err)
	}
	peers := fr.GetPeers()
	if len(peers) != 1 || peers[0] != "http://peer1.example.com" {
		t.Fatalf("expected [http://peer1.example.com], got %v", peers)
	}

	// Duplicate should error
	if err := fr.AddPeer("http://peer1.example.com"); err == nil {
		t.Fatal("expected error for duplicate peer")
	}
}

func TestRemovePeer(t *testing.T) {
	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)

	fr.AddPeer("http://peer1.example.com")
	fr.AddPeer("http://peer2.example.com")

	if err := fr.RemovePeer("http://peer1.example.com"); err != nil {
		t.Fatalf("RemovePeer failed: %v", err)
	}
	peers := fr.GetPeers()
	if len(peers) != 1 || peers[0] != "http://peer2.example.com" {
		t.Fatalf("expected [http://peer2.example.com], got %v", peers)
	}

	// Remove non-existent should error
	if err := fr.RemovePeer("http://nonexistent.example.com"); err == nil {
		t.Fatal("expected error for removing non-existent peer")
	}
}

func TestGetPeersReturnsCopy(t *testing.T) {
	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)

	fr.AddPeer("http://peer1.example.com")
	peers := fr.GetPeers()
	peers[0] = "http://modified.example.com"

	original := fr.GetPeers()
	if original[0] != "http://peer1.example.com" {
		t.Fatal("GetPeers should return a copy, not a reference")
	}
}

func TestBroadcastEvent(t *testing.T) {
	var receivedMu sync.Mutex
	var receivedEvent *nostr.Event
	callCount := 0

	// Mock peer server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nostr/broadcast" {
			var req broadcastRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode broadcast request: %v", err)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			receivedMu.Lock()
			receivedEvent = &req.Event
			callCount++
			receivedMu.Unlock()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)
	fr.AddPeer(server.URL)

	event := nostr.Event{
		ID:        "test-event-1",
		PubKey:    "testpubkey",
		Kind:      1,
		Content:   "Hello federation!",
		CreatedAt: 1700000000,
	}

	count, err := fr.BroadcastEvent(event)
	if err != nil {
		t.Fatalf("BroadcastEvent failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 successful broadcast, got %d", count)
	}

	receivedMu.Lock()
	if receivedEvent == nil || receivedEvent.ID != "test-event-1" {
		t.Fatalf("expected event ID test-event-1, got %v", receivedEvent)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
	receivedMu.Unlock()
}

func TestBroadcastEventNoPeers(t *testing.T) {
	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)

	event := nostr.Event{ID: "test-1", Content: "hello"}
	count, err := fr.BroadcastEvent(event)
	if err != nil {
		t.Fatalf("expected no error with no peers, got: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 count, got %d", count)
	}
}

func TestBroadcastEventPeerError(t *testing.T) {
	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)
	fr.AddPeer("http://127.0.0.1:1") // unreachable

	event := nostr.Event{ID: "test-1", Content: "hello"}
	count, err := fr.BroadcastEvent(event)
	if err == nil {
		t.Fatal("expected error when peer is unreachable")
	}
	if count != 0 {
		t.Fatalf("expected 0 successful broadcasts, got %d", count)
	}
}

func TestFetchRemoteEvents(t *testing.T) {
	events := []nostr.Event{
		{ID: "evt-1", Content: "hello", CreatedAt: 1700000001},
		{ID: "evt-2", Content: "world", CreatedAt: 1700000002},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nostr/query" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(syncResponse{Events: events})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)

	filter := nostr.Filter{Limit: 100}
	result, err := fr.FetchRemoteEvents(server.URL, filter)
	if err != nil {
		t.Fatalf("FetchRemoteEvents failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result))
	}
	if result[0].ID != "evt-1" || result[1].ID != "evt-2" {
		t.Fatalf("unexpected events: %v", result)
	}
}

func TestFetchRemoteEventsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)

	filter := nostr.Filter{Limit: 100}
	_, err := fr.FetchRemoteEvents(server.URL, filter)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestSyncEvents(t *testing.T) {
	events := []nostr.Event{
		{ID: "sync-1", Content: "synced event 1", CreatedAt: 1700000010},
		{ID: "sync-2", Content: "synced event 2", CreatedAt: 1700000020},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nostr/query" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(syncResponse{Events: events})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)
	fr.AddPeer(server.URL)

	err := fr.SyncEvents(context.Background(), 1700000000)
	if err != nil {
		t.Fatalf("SyncEvents failed: %v", err)
	}
}

func TestSyncEventsNoPeers(t *testing.T) {
	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)

	err := fr.SyncEvents(context.Background(), 0)
	if err != nil {
		t.Fatalf("expected no error with no peers, got: %v", err)
	}
}

func TestConcurrentPeerOperations(t *testing.T) {
	relay := nostr.NewRelay(1000, nil)
	fr := NewFederatedRelay(relay)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			fr.AddPeer(fmt.Sprintf("http://peer%d.example.com", i))
		}(i)
	}
	wg.Wait()

	if len(fr.GetPeers()) != 100 {
		t.Fatalf("expected 100 peers, got %d", len(fr.GetPeers()))
	}
}
