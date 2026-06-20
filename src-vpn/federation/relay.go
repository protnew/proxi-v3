// Package federation implements relay federation for distributing Nostr events
// across multiple relay instances. It provides peer management, event broadcasting,
// remote event fetching, and full synchronization capabilities.
package federation

import (
	"context"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/unkillable-messenger/vpn/nostr"
)

// FederatedRelay manages communication with remote relay peers.
// It wraps a local nostr.Relay and adds federation capabilities.
type FederatedRelay struct {
	mu         sync.RWMutex
	localRelay *nostr.Relay
	peers      []string
	httpClient *http.Client
}

// FederatedPeer represents a persisted peer entry.
type FederatedPeer struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	LastSync int64  `json:"lastSync"`
	Status   string `json:"status"` // "active", "inactive", "error"
}

// broadcastRequest is the JSON body sent to peer relays.
type broadcastRequest struct {
	Event nostr.Event `json:"event"`
}

// syncResponse is the JSON response from a peer relay sync endpoint.
type syncResponse struct {
	Events []nostr.Event `json:"events"`
}

// NewFederatedRelay creates a new FederatedRelay wrapping the given local relay.
func NewFederatedRelay(local *nostr.Relay) *FederatedRelay {
	return &FederatedRelay{
		localRelay: local,
		peers:      make([]string, 0),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AddPeer adds a remote relay URL to the federation peer list.
// Returns an error if the peer is already present.
func (f *FederatedRelay) AddPeer(peerURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, p := range f.peers {
		if p == peerURL {
			return fmt.Errorf("peer %s already exists", peerURL)
		}
	}
	f.peers = append(f.peers, peerURL)
	log.Printf("[federation] peer added: %s", peerURL)
	return nil
}

// RemovePeer removes a remote relay URL from the federation peer list.
// Returns an error if the peer was not found.
func (f *FederatedRelay) RemovePeer(peerURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, p := range f.peers {
		if p == peerURL {
			f.peers = append(f.peers[:i], f.peers[i+1:]...)
			log.Printf("[federation] peer removed: %s", peerURL)
			return nil
		}
	}
	return fmt.Errorf("peer %s not found", peerURL)
}

// GetPeers returns a copy of the current peer list.
func (f *FederatedRelay) GetPeers() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]string, len(f.peers))
	copy(result, f.peers)
	return result
}

// BroadcastEvent sends an event to all peer relays via HTTP POST /nostr/broadcast.
// Errors from individual peers are logged but do not stop broadcasting to others.
// Returns the number of peers that accepted the event and the first error encountered.
func (f *FederatedRelay) BroadcastEvent(event nostr.Event) (int, error) {
	f.mu.RLock()
	peers := make([]string, len(f.peers))
	copy(peers, f.peers)
	f.mu.RUnlock()

	if len(peers) == 0 {
		return 0, nil
	}

	body, err := json.Marshal(broadcastRequest{Event: event})
	if err != nil {
		return 0, fmt.Errorf("marshal event: %w", err)
	}

	var firstErr error
	successCount := 0

	for _, peer := range peers {
		url := peer + "/nostr/broadcast"
		resp, err := f.httpClient.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("[federation] broadcast to %s failed: %v", peer, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("broadcast to %s: %w", peer, err)
			}
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Printf("[federation] broadcast to %s returned status %d", peer, resp.StatusCode)
			if firstErr == nil {
				firstErr = fmt.Errorf("broadcast to %s: HTTP %d", peer, resp.StatusCode)
			}
			continue
		}
		successCount++
	}

	return successCount, firstErr
}

// FetchRemoteEvents queries events from a specific peer relay that match the filter.
// It sends a POST request to /nostr/query on the peer.
func (f *FederatedRelay) FetchRemoteEvents(peerURL string, filter nostr.Filter) ([]nostr.Event, error) {
	body, err := json.Marshal(filter)
	if err != nil {
		return nil, fmt.Errorf("marshal filter: %w", err)
	}

	url := peerURL + "/nostr/query"
	resp, err := f.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("fetch from %s: %w", peerURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch from %s: HTTP %d: %s", peerURL, resp.StatusCode, string(respBody))
	}

	var result syncResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response from %s: %w", peerURL, err)
	}

	return result.Events, nil
}

// SyncEvents performs a full synchronization with all peer relays.
// It fetches all events from each peer that were created after the given timestamp
// and logs the results. Events are fetched via POST /nostr/query with a since filter.
func (f *FederatedRelay) SyncEvents(ctx context.Context, since int64) error {
	f.mu.RLock()
	peers := make([]string, len(f.peers))
	copy(peers, f.peers)
	f.mu.RUnlock()

	if len(peers) == 0 {
		return nil
	}

	var firstErr error
	syncSince := since

	for _, peer := range peers {
		filter := nostr.Filter{
			Since: &syncSince,
			Limit: 5000,
		}

		events, err := f.FetchRemoteEvents(peer, filter)
		if err != nil {
			log.Printf("[federation] sync with %s failed: %v", peer, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("sync with %s: %w", peer, err)
			}
			continue
		}

		log.Printf("[federation] synced %d events from %s (since=%d)", len(events), peer, since)
	}

	return firstErr
}
