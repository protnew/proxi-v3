package nostr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// ==================== NIP-01 Types ====================

// Event is a Nostr event (NIP-01).
type Event struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

// Filter is a NIP-01 subscription filter.
type Filter struct {
	IDs     []string `json:"ids,omitempty"`
	Authors []string `json:"authors,omitempty"`
	Kinds   []int    `json:"kinds,omitempty"`
	Since   *int64   `json:"since,omitempty"`
	Until   *int64   `json:"until,omitempty"`
	Limit   int      `json:"limit,omitempty"`
	// Tag filters: #e, #p, etc.
	TagFilters map[string][]string `json:"-"`
}

// Subscription links a client to a filter.
type Subscription struct {
	ID     string
	Filter Filter
	Client *Client
}

// Client represents a connected WebSocket client.
type Client struct {
	conn          WebSocketConn
	mu            sync.Mutex
	subscriptions map[string]*Subscription
	// AuthPubkey — x-only hex identity taken from the JWT at connection time.
	// Empty = unauthenticated connection (tests / auth-disabled relay).
	AuthPubkey string
}

// WebSocketConn abstracts gorilla/nhooyr websocket.
type WebSocketConn interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	Close() error
}

// DBProvider defines the interface the relay needs for event persistence.
type DBProvider interface {
	SaveNostrEvent(evt store.NostrEvent) error
	GetNostrEvents(filter store.NostrEventFilter) ([]store.NostrEvent, error)
}

// Relay is a NIP-01 relay.
type Relay struct {
	mu        sync.RWMutex
	events    []Event // kept for backward-compatible GetStats and in-memory fallback
	clients   map[*Client]bool
	maxEvents int
	db        DBProvider

	// OnEvent is called after a valid event is accepted (optional).
	OnEvent func(Event)
}

// NewRelay creates a new NIP-01 relay.
// If db is non-nil, events are persisted to SQLite; otherwise falls back to in-memory only.
func NewRelay(maxEvents int, db DBProvider) *Relay {
	if maxEvents <= 0 {
		maxEvents = 50000
	}
	return &Relay{
		events:    make([]Event, 0),
		clients:   make(map[*Client]bool),
		maxEvents: maxEvents,
		db:        db,
	}
}

// ComputeEventID computes the event ID per NIP-01.
func ComputeEventID(e *Event) string {
	// Serialize [0, "pubkey", created_at, kind, tags, content]
	data := []interface{}{
		0,
		e.PubKey,
		e.CreatedAt,
		e.Kind,
		e.Tags,
		e.Content,
	}
	raw, _ := json.Marshal(data)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

// HandleClient handles a WebSocket client connection (NIP-01 protocol).
func (r *Relay) HandleClient(conn WebSocketConn) {
	r.HandleClientAuth(conn, "")
}

// HandleClientAuth handles an authenticated WebSocket client: authPubkey is the
// x-only hex pubkey from the connection's JWT. Events published on this
// connection must be signed AND match this identity (P1: no publishing as
// someone else).
func (r *Relay) HandleClientAuth(conn WebSocketConn, authPubkey string) {
	client := &Client{
		conn:          conn,
		subscriptions: make(map[string]*Subscription),
		AuthPubkey:    strings.ToLower(authPubkey),
	}

	r.mu.Lock()
	r.clients[client] = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.clients, client)
		r.mu.Unlock()
		client.conn.Close()
	}()

	for {
		var msg []json.RawMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		if len(msg) == 0 {
			continue
		}

		var cmd string
		json.Unmarshal(msg[0], &cmd)

		switch cmd {
		case "EVENT":
			r.handleEvent(client, msg[1:])
		case "REQ":
			r.handleReq(client, msg[1:])
		case "CLOSE":
			r.handleClose(client, msg[1:])
		}
	}
}

// handleEvent processes an EVENT message from a client.
func (r *Relay) handleEvent(client *Client, raw []json.RawMessage) {
	if len(raw) == 0 {
		client.sendNotice("empty event")
		return
	}

	var event Event
	if err := json.Unmarshal(raw[0], &event); err != nil {
		client.sendNotice("invalid event: " + err.Error())
		return
	}

	// Compute and verify ID
	expectedID := ComputeEventID(&event)
	if event.ID != expectedID {
		client.sendOK(event.ID, false, "invalid event id")
		return
	}

	// P1: schnorr signature is mandatory for every WS-published event.
	if err := verifyEventSig(&event); err != nil {
		client.sendOK(event.ID, false, "invalid signature: "+err.Error())
		return
	}
	// P1: authenticated connections may only publish as their own identity.
	if client.AuthPubkey != "" && !strings.EqualFold(event.PubKey, client.AuthPubkey) {
		client.sendOK(event.ID, false, "pubkey does not match connection identity")
		return
	}

	// Persist to SQLite if db is configured
	if r.db != nil {
		if err := r.db.SaveNostrEvent(store.NostrEvent{
			ID:        event.ID,
			PubKey:    event.PubKey,
			Kind:      event.Kind,
			Tags:      event.Tags,
			Content:   event.Content,
			Sig:       event.Sig,
			CreatedAt: event.CreatedAt,
		}); err != nil {
			log.Printf("[nostr] failed to save event %s: %v", event.ID, err)
		}
	}

	// Also keep in-memory for stats / backward compat
	r.mu.Lock()
	r.events = append(r.events, event)
	if len(r.events) > r.maxEvents {
		r.events = r.events[len(r.events)-r.maxEvents:]
	}
	// Broadcast to matching subscriptions
	subs := make([]*Subscription, 0)
	for c := range r.clients {
		for _, sub := range c.subscriptions {
			if matchFilter(&event, &sub.Filter) && deliveryAllowed(c, &event) {
				subs = append(subs, sub)
			}
		}
	}
	r.mu.Unlock()

	// Send OK to sender
	client.sendOK(event.ID, true, "")

	// Optional app hook (VPN signaling, etc.)
	if r.OnEvent != nil {
		go r.OnEvent(event)
	}

	// Forward to matching subscribers
	for _, sub := range subs {
		msg := []interface{}{"EVENT", sub.ID, event}
		sub.Client.send(msg)
	}
}

// handleReq processes a REQ (subscribe) message.
func (r *Relay) handleReq(client *Client, raw []json.RawMessage) {
	if len(raw) < 2 {
		client.sendNotice("REQ requires sub_id and filter")
		return
	}

	var subID string
	json.Unmarshal(raw[0], &subID)

	var filter Filter
	json.Unmarshal(raw[1], &filter)

	// P1: DM (kind 4) and VPN signaling (kind 30090) are private — a
	// subscription may only ever see its own identity's events.
	if containsSensitiveKind(filter.Kinds) {
		if client.AuthPubkey == "" {
			client.sendNotice("auth required for kinds 4/30090")
			return
		}
		filter.Authors = []string{client.AuthPubkey}
	}

	sub := &Subscription{
		ID:     subID,
		Filter: filter,
		Client: client,
	}

	client.mu.Lock()
	client.subscriptions[subID] = sub
	client.mu.Unlock()

	// Load events from SQLite if db is configured, otherwise from in-memory
	if r.db != nil {
		limit := filter.Limit
		if limit <= 0 {
			limit = 100
		}
		dbFilter := store.NostrEventFilter{
			IDs:     filter.IDs,
			Kinds:   filter.Kinds,
			Authors: filter.Authors,
			Since:   filter.Since,
			Until:   filter.Until,
			Limit:   limit,
		}
		storedEvents, err := r.db.GetNostrEvents(dbFilter)
		if err != nil {
			log.Printf("[nostr] failed to query events for sub %s: %v", subID, err)
		} else {
			// Results come DESC (newest first); send in order for client
			for i := len(storedEvents) - 1; i >= 0; i-- {
				se := storedEvents[i]
				evt := Event{
					ID:        se.ID,
					PubKey:    se.PubKey,
					Kind:      se.Kind,
					Tags:      se.Tags,
					Content:   se.Content,
					Sig:       se.Sig,
					CreatedAt: se.CreatedAt,
				}
				msg := []interface{}{"EVENT", subID, evt}
				client.send(msg)
			}
		}
	} else {
		// Fallback: in-memory (original behavior)
		r.mu.RLock()
		matched := 0
		limit := filter.Limit
		if limit <= 0 {
			limit = 100
		}
		for i := len(r.events) - 1; i >= 0 && matched < limit; i-- {
			if matchFilter(&r.events[i], &filter) {
				msg := []interface{}{"EVENT", subID, r.events[i]}
				client.send(msg)
				matched++
			}
		}
		r.mu.RUnlock()
	}

	// Send EOSE (End of Stored Events)
	client.send([]string{"EOSE", subID})
}

// handleClose processes a CLOSE message.
func (r *Relay) handleClose(client *Client, raw []json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var subID string
	json.Unmarshal(raw[0], &subID)

	client.mu.Lock()
	delete(client.subscriptions, subID)
	client.mu.Unlock()
}

// verifyEventSig checks the schnorr signature of an event over its ID (NIP-01).


func (c *Client) send(v interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn.WriteJSON(v)
}

func (c *Client) sendNotice(msg string) {
	c.send([]string{"NOTICE", msg})
}

func (c *Client) sendOK(eventID string, ok bool, reason string) {
	c.send([]interface{}{"OK", eventID, ok, reason})
}

// ==================== HTTP Handler ====================

// RelayHTTPHandler returns an HTTP handler that upgrades to WebSocket and handles NIP-01.
// This wraps the relay for integration with the existing webserver.
func (r *Relay) RelayHTTPHandler(upgrader interface {
	Upgrade(w http.ResponseWriter, req *http.Request) (WebSocketConn, error)
}) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req)
		if err != nil {
			log.Printf("[nostr] upgrade error: %v", err)
			return
		}
		r.HandleClient(conn)
	}
}

// GetStats returns relay statistics.
func (r *Relay) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return map[string]interface{}{
		"events":     len(r.events),
		"clients":    len(r.clients),
		"max_events": r.maxEvents,
		"uptime":     fmt.Sprintf("%d", time.Now().Unix()),
	}
}


// InjectLocalEvent publishes a server-side event into the relay pipeline (INF-010).
// Used for VPN signaling without a remote WS client. Skips ID/sig crypto verification
// (local trust boundary). Still persists, broadcasts to subscribers, fires OnEvent.
func (r *Relay) InjectLocalEvent(event Event) {
	// R11/SigLocal: local inject bypasses schnorr; Sig=="local" must never be treated as authenticated Nostr events.

	if event.CreatedAt == 0 {
		event.CreatedAt = time.Now().Unix()
	}
	if event.ID == "" {
		event.ID = ComputeEventID(&event)
	}
	// R11: Sig=="local" is an unsafe local-only marker — never persist as a Nostr event.
	if event.Sig != "local" && r.db != nil {
		if err := r.db.SaveNostrEvent(store.NostrEvent{
			ID: event.ID, PubKey: event.PubKey, Kind: event.Kind,
			Tags: event.Tags, Content: event.Content, Sig: event.Sig, CreatedAt: event.CreatedAt,
		}); err != nil {
			log.Printf("[nostr] InjectLocalEvent save: %v", err)
		}
	}
	r.mu.Lock()
	r.events = append(r.events, event)
	if len(r.events) > r.maxEvents {
		r.events = r.events[len(r.events)-r.maxEvents:]
	}
	subs := make([]*Subscription, 0)
	for c := range r.clients {
		for _, sub := range c.subscriptions {
			if matchFilter(&event, &sub.Filter) && deliveryAllowed(c, &event) {
				subs = append(subs, sub)
			}
		}
	}
	r.mu.Unlock()
	if r.OnEvent != nil {
		go r.OnEvent(event)
	}
	for _, sub := range subs {
		sub.Client.send([]interface{}{"EVENT", sub.ID, event})
	}
}
