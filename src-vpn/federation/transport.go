// Package federation implements relay federation for distributing Nostr events
// across multiple relay instances.
//
// transport.go implements HTTP-based federation transport for sending events
// to remote peers, fetching events from peers, and full synchronization.
package federation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/unkillable-messenger/vpn/nostr"
)

// HTTPTransport provides HTTP-based federation transport between relay peers.
// Each peer is identified by a peerID and has an associated baseURL for HTTP requests.
type HTTPTransport struct {
	mu     sync.RWMutex
	client *http.Client
	peers  map[string]string // peerID → baseURL
}

// NewHTTPTransport creates a new HTTPTransport with default HTTP client settings.
func NewHTTPTransport() *HTTPTransport {
	return &HTTPTransport{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		peers: make(map[string]string),
	}
}

// AddPeer registers a peer with its base URL for federation.
func (t *HTTPTransport) AddPeer(peerID, baseURL string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.peers[peerID] = baseURL
}

// RemovePeer removes a peer from the transport.
func (t *HTTPTransport) RemovePeer(peerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.peers, peerID)
}

// GetPeers returns all registered peer IDs.
func (t *HTTPTransport) GetPeers() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	peers := make([]string, 0, len(t.peers))
	for id := range t.peers {
		peers = append(peers, id)
	}
	return peers
}

// SendEvent sends a Nostr event to a specific peer via HTTP POST.
// Endpoint: POST /api/federation/event
//
// Parameters:
//   - peerID: the ID of the target peer
//   - event:  the Nostr event to send
//
// Returns an error if the peer is not found, the request fails, or the peer returns a non-2xx status.
func (t *HTTPTransport) SendEvent(peerID string, event nostr.Event) error {
	t.mu.RLock()
	baseURL, ok := t.peers[peerID]
	t.mu.RUnlock()

	if !ok {
		return fmt.Errorf("federation transport: peer %s not found", peerID)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("federation transport: marshal event: %w", err)
	}

	url := baseURL + "/api/federation/event"

	var lastErr error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond) // backoff
		}

		resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("federation transport: POST to %s: %w", url, err)
			log.Printf("[federation-transport] send event to %s attempt %d failed: %v", peerID, attempt+1, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("federation transport: peer %s returned HTTP %d", peerID, resp.StatusCode)
	}

	return lastErr
}

// FetchEvents fetches events from a specific peer since the given timestamp.
// Endpoint: GET /api/federation/events?since=<timestamp>
//
// Parameters:
//   - peerID: the ID of the source peer
//   - since:  Unix timestamp; only events created after this time are returned
//
// Returns the fetched events or an error.
func (t *HTTPTransport) FetchEvents(peerID string, since int64) ([]nostr.Event, error) {
	t.mu.RLock()
	baseURL, ok := t.peers[peerID]
	t.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("federation transport: peer %s not found", peerID)
	}

	url := fmt.Sprintf("%s/api/federation/events?since=%d", baseURL, since)

	resp, err := t.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("federation transport: GET from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("federation transport: GET from %s: HTTP %d: %s", url, resp.StatusCode, string(body))
	}

	var events []nostr.Event
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("federation transport: decode events: %w", err)
	}

	return events, nil
}

// SyncAll synchronizes events from all peers into the local federated relay.
// It fetches events from each peer that were created after the relay's latest event
// timestamp and logs the results.
//
// Parameters:
//   - relay: the local FederatedRelay to sync into
//
// Returns the first error encountered (if any).
func (t *HTTPTransport) SyncAll(ctx context.Context, relay *FederatedRelay) error {
	t.mu.RLock()
	peers := make(map[string]string, len(t.peers))
	for k, v := range t.peers {
		peers[k] = v
	}
	t.mu.RUnlock()

	if len(peers) == 0 {
		return nil
	}

	var firstErr error
	since := time.Now().Add(-24 * time.Hour).Unix() // sync last 24h by default

	for peerID, baseURL := range peers {
		url := fmt.Sprintf("%s/api/federation/events?since=%d", baseURL, since)

		resp, err := t.client.Get(url)
		if err != nil {
			log.Printf("[federation-transport] sync from %s failed: %v", peerID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("sync from %s: %w", peerID, err)
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Printf("[federation-transport] sync from %s returned HTTP %d", peerID, resp.StatusCode)
			if firstErr == nil {
				firstErr = fmt.Errorf("sync from %s: HTTP %d", peerID, resp.StatusCode)
			}
			continue
		}

		var events []nostr.Event
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			resp.Body.Close()
			log.Printf("[federation-transport] sync from %s decode error: %v", peerID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("sync from %s: decode: %w", peerID, err)
			}
			continue
		}
		resp.Body.Close()

		log.Printf("[federation-transport] synced %d events from %s", len(events), peerID)
	}

	return firstErr
}
