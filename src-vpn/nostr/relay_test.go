package nostr

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

// ==================== Mocks ====================

// mockDBProvider is a mock implementation of DBProvider.
type mockDBProvider struct {
	mu     sync.Mutex
	events []store.NostrEvent
	err    error // if set, Save/Get return this error
}

func (m *mockDBProvider) SaveNostrEvent(evt store.NostrEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, evt)
	return nil
}

func (m *mockDBProvider) GetNostrEvents(filter store.NostrEventFilter) ([]store.NostrEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	var result []store.NostrEvent
	for _, evt := range m.events {
		if matchNostrEvent(evt, filter) {
			result = append(result, evt)
		}
	}
	// Apply limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result, nil
}

// matchNostrEvent is a simple filter matcher for the mock.
func matchNostrEvent(evt store.NostrEvent, f store.NostrEventFilter) bool {
	if len(f.IDs) > 0 && !containsString(f.IDs, evt.ID) {
		return false
	}
	if len(f.Authors) > 0 && !containsString(f.Authors, evt.PubKey) {
		return false
	}
	if len(f.Kinds) > 0 && !containsInt2(f.Kinds, evt.Kind) {
		return false
	}
	if f.Since != nil && evt.CreatedAt < *f.Since {
		return false
	}
	if f.Until != nil && evt.CreatedAt > *f.Until {
		return false
	}
	return true
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func containsInt2(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// mockConn is a mock WebSocketConn that records written messages.
type mockConn struct {
	mu        sync.Mutex
	written   []interface{}
	readCh    chan []json.RawMessage
	closeErr  error
	closed    bool
}

func newMockConn() *mockConn {
	return &mockConn{
		readCh: make(chan []json.RawMessage, 16),
	}
}

func (c *mockConn) ReadJSON(v interface{}) error {
	msg, ok := <-c.readCh
	if !ok {
		return fmt.Errorf("closed")
	}
	raw, _ := json.Marshal(msg)
	return json.Unmarshal(raw, v)
}

func (c *mockConn) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.written = append(c.written, v)
	return nil
}

func (c *mockConn) Close() error {
	c.closed = true
	return c.closeErr
}

func (c *mockConn) getWritten() []interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]interface{}, len(c.written))
	copy(result, c.written)
	return result
}

// ==================== Tests ====================

func TestNewRelay(t *testing.T) {
	t.Run("with positive maxEvents", func(t *testing.T) {
		r := NewRelay(1000, nil)
		if r == nil {
			t.Fatal("expected non-nil relay")
		}
		if r.maxEvents != 1000 {
			t.Errorf("expected maxEvents=1000, got %d", r.maxEvents)
		}
		if r.events == nil {
			t.Error("expected events slice to be initialized")
		}
		if r.clients == nil {
			t.Error("expected clients map to be initialized")
		}
		if r.db != nil {
			t.Error("expected db to be nil")
		}
	})

	t.Run("with zero maxEvents defaults to 50000", func(t *testing.T) {
		r := NewRelay(0, nil)
		if r.maxEvents != 50000 {
			t.Errorf("expected maxEvents=50000, got %d", r.maxEvents)
		}
	})

	t.Run("with negative maxEvents defaults to 50000", func(t *testing.T) {
		r := NewRelay(-5, nil)
		if r.maxEvents != 50000 {
			t.Errorf("expected maxEvents=50000, got %d", r.maxEvents)
		}
	})

	t.Run("with DBProvider", func(t *testing.T) {
		db := &mockDBProvider{}
		r := NewRelay(100, db)
		if r.db == nil {
			t.Error("expected db to be set")
		}
	})
}

func TestComputeEventID(t *testing.T) {
	// Known event — compute ID and verify it's deterministic
	evt := &Event{
		PubKey:    "testpubkey",
		CreatedAt: 1234567890,
		Kind:      1,
		Tags:      [][]string{{"e", "referredevent"}, {"p", "referredpubkey"}},
		Content:   "hello world",
	}

	id1 := ComputeEventID(evt)
	id2 := ComputeEventID(evt)

	if id1 != id2 {
		t.Errorf("ComputeEventID is not deterministic: %s != %s", id1, id2)
	}
	if len(id1) != 64 {
		t.Errorf("expected 64-char hex ID, got %d chars", len(id1))
	}

	t.Run("different content produces different ID", func(t *testing.T) {
		evt2 := &Event{
			PubKey:    "testpubkey",
			CreatedAt: 1234567890,
			Kind:      1,
			Tags:      nil,
			Content:   "different content",
		}
		id3 := ComputeEventID(evt2)
		if id1 == id3 {
			t.Error("expected different IDs for different events")
		}
	})

	t.Run("empty event produces valid hash", func(t *testing.T) {
		empty := &Event{}
		id := ComputeEventID(empty)
		if len(id) != 64 {
			t.Errorf("expected 64-char hex ID for empty event, got %d", len(id))
		}
	})
}

func TestMatchFilter_IDs(t *testing.T) {
	evt := &Event{ID: "abc123", PubKey: "pk1", Kind: 1, CreatedAt: 100}

	t.Run("matching ID", func(t *testing.T) {
		f := &Filter{IDs: []string{"abc123", "def456"}}
		if !matchFilter(evt, f) {
			t.Error("expected event to match filter with matching ID")
		}
	})

	t.Run("non-matching ID", func(t *testing.T) {
		f := &Filter{IDs: []string{"xyz", "def456"}}
		if matchFilter(evt, f) {
			t.Error("expected event NOT to match filter with different IDs")
		}
	})

	t.Run("empty IDs filter matches all", func(t *testing.T) {
		f := &Filter{IDs: nil}
		if !matchFilter(evt, f) {
			t.Error("expected event to match empty IDs filter")
		}
	})
}

func TestMatchFilter_Authors(t *testing.T) {
	evt := &Event{ID: "id1", PubKey: "author1", Kind: 1, CreatedAt: 100}

	t.Run("matching author", func(t *testing.T) {
		f := &Filter{Authors: []string{"author1", "author2"}}
		if !matchFilter(evt, f) {
			t.Error("expected event to match filter with matching author")
		}
	})

	t.Run("non-matching author", func(t *testing.T) {
		f := &Filter{Authors: []string{"author2", "author3"}}
		if matchFilter(evt, f) {
			t.Error("expected event NOT to match filter with different authors")
		}
	})

	t.Run("empty authors filter matches all", func(t *testing.T) {
		f := &Filter{Authors: nil}
		if !matchFilter(evt, f) {
			t.Error("expected event to match empty authors filter")
		}
	})
}

func TestMatchFilter_Kinds(t *testing.T) {
	evt := &Event{ID: "id1", PubKey: "pk1", Kind: 1, CreatedAt: 100}

	t.Run("matching kind", func(t *testing.T) {
		f := &Filter{Kinds: []int{0, 1}}
		if !matchFilter(evt, f) {
			t.Error("expected event to match filter with kind 1")
		}
	})

	t.Run("non-matching kind", func(t *testing.T) {
		f := &Filter{Kinds: []int{0, 3}}
		if matchFilter(evt, f) {
			t.Error("expected event NOT to match filter without kind 1")
		}
	})

	t.Run("empty kinds filter matches all", func(t *testing.T) {
		f := &Filter{Kinds: nil}
		if !matchFilter(evt, f) {
			t.Error("expected event to match empty kinds filter")
		}
	})
}

func TestMatchFilter_Since(t *testing.T) {
	evt := &Event{ID: "id1", PubKey: "pk1", Kind: 1, CreatedAt: 100}

	t.Run("event after since", func(t *testing.T) {
		since := int64(50)
		f := &Filter{Since: &since}
		if !matchFilter(evt, f) {
			t.Error("expected event to match when created_at >= since")
		}
	})

	t.Run("event before since", func(t *testing.T) {
		since := int64(200)
		f := &Filter{Since: &since}
		if matchFilter(evt, f) {
			t.Error("expected event NOT to match when created_at < since")
		}
	})

	t.Run("event exactly at since", func(t *testing.T) {
		since := int64(100)
		f := &Filter{Since: &since}
		if !matchFilter(evt, f) {
			t.Error("expected event to match when created_at == since")
		}
	})

	t.Run("nil since matches all", func(t *testing.T) {
		f := &Filter{Since: nil}
		if !matchFilter(evt, f) {
			t.Error("expected event to match nil since filter")
		}
	})
}

func TestMatchFilter_Until(t *testing.T) {
	evt := &Event{ID: "id1", PubKey: "pk1", Kind: 1, CreatedAt: 100}

	t.Run("event before until", func(t *testing.T) {
		until := int64(200)
		f := &Filter{Until: &until}
		if !matchFilter(evt, f) {
			t.Error("expected event to match when created_at <= until")
		}
	})

	t.Run("event after until", func(t *testing.T) {
		until := int64(50)
		f := &Filter{Until: &until}
		if matchFilter(evt, f) {
			t.Error("expected event NOT to match when created_at > until")
		}
	})

	t.Run("event exactly at until", func(t *testing.T) {
		until := int64(100)
		f := &Filter{Until: &until}
		if !matchFilter(evt, f) {
			t.Error("expected event to match when created_at == until")
		}
	})

	t.Run("nil until matches all", func(t *testing.T) {
		f := &Filter{Until: nil}
		if !matchFilter(evt, f) {
			t.Error("expected event to match nil until filter")
		}
	})
}

func TestHandleEvent(t *testing.T) {
	// Helper to create a valid event with computed ID
	makeEvent := func(pubkey string, kind int, content string, createdAt int64) Event {
		evt := Event{
			PubKey:    pubkey,
			Kind:      kind,
			Content:   content,
			CreatedAt: createdAt,
			Tags:      nil,
			Sig:       "fakesig",
		}
		evt.ID = ComputeEventID(&evt)
		return evt
	}

	t.Run("valid event is stored and OK sent", func(t *testing.T) {
		db := &mockDBProvider{}
		r := NewRelay(1000, db)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		r.mu.Lock()
		r.clients[client] = true
		r.mu.Unlock()

		evt := makeEvent("pk1", 1, "hello", 1000)
		raw, _ := json.Marshal(evt)

		r.handleEvent(client, []json.RawMessage{raw})

		written := conn.getWritten()
		if len(written) == 0 {
			t.Fatal("expected some messages written to conn")
		}
		// Last message should be OK
		last := written[len(written)-1]
		okMsg, ok := last.([]interface{})
		if !ok {
			t.Fatalf("expected []interface{}, got %T", last)
		}
		if len(okMsg) < 3 {
			t.Fatalf("expected at least 3 elements in OK message, got %d", len(okMsg))
		}
		if okMsg[0] != "OK" {
			t.Errorf("expected OK, got %v", okMsg[0])
		}
		if okMsg[2] != true {
			t.Errorf("expected ok=true, got %v", okMsg[2])
		}

		// Verify saved to DB
		db.mu.Lock()
		if len(db.events) != 1 {
			t.Errorf("expected 1 event in DB, got %d", len(db.events))
		}
		db.mu.Unlock()

		// Verify saved in-memory
		r.mu.RLock()
		if len(r.events) != 1 {
			t.Errorf("expected 1 in-memory event, got %d", len(r.events))
		}
		r.mu.RUnlock()
	})

	t.Run("empty raw sends notice", func(t *testing.T) {
		r := NewRelay(100, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		r.handleEvent(client, []json.RawMessage{})

		written := conn.getWritten()
		if len(written) != 1 {
			t.Fatalf("expected 1 message, got %d", len(written))
		}
		msg, ok := written[0].([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", written[0])
		}
		if msg[0] != "NOTICE" || msg[1] != "empty event" {
			t.Errorf("expected NOTICE 'empty event', got %v", msg)
		}
	})

	t.Run("invalid JSON sends Notice", func(t *testing.T) {
		r := NewRelay(100, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		r.handleEvent(client, []json.RawMessage{json.RawMessage(`{invalid json`)})

		written := conn.getWritten()
		if len(written) != 1 {
			t.Fatalf("expected 1 message, got %d", len(written))
		}
		msg, ok := written[0].([]string)
		if !ok || msg[0] != "NOTICE" {
			t.Errorf("expected NOTICE for invalid JSON, got %v", written[0])
		}
	})

	t.Run("invalid event ID sends OK false", func(t *testing.T) {
		r := NewRelay(100, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		evt := Event{
			ID: "wrongid", PubKey: "pk1", Kind: 1, Content: "test", CreatedAt: 100, Sig: "sig",
		}
		raw, _ := json.Marshal(evt)

		r.handleEvent(client, []json.RawMessage{raw})

		written := conn.getWritten()
		if len(written) != 1 {
			t.Fatalf("expected 1 message, got %d", len(written))
		}
		okMsg, ok := written[0].([]interface{})
		if !ok || okMsg[0] != "OK" {
			t.Fatalf("expected OK message, got %v", written[0])
		}
		if okMsg[2] != false {
			t.Errorf("expected ok=false for invalid ID, got %v", okMsg[2])
		}
	})

	t.Run("broadcasts to matching subscribers", func(t *testing.T) {
		r := NewRelay(1000, nil)
		// Subscriber client with a matching filter
		subConn := newMockConn()
		subClient := &Client{conn: subConn, subscriptions: make(map[string]*Subscription)}
		subClient.subscriptions["sub1"] = &Subscription{
			ID:     "sub1",
			Filter: Filter{Kinds: []int{1}},
			Client: subClient,
		}

		// Sender client
		senderConn := newMockConn()
		senderClient := &Client{conn: senderConn, subscriptions: make(map[string]*Subscription)}

		r.mu.Lock()
		r.clients[senderClient] = true
		r.clients[subClient] = true
		r.mu.Unlock()

		evt := makeEvent("pk1", 1, "broadcast test", 1000)
		raw, _ := json.Marshal(evt)

		r.handleEvent(senderClient, []json.RawMessage{raw})

		subWritten := subConn.getWritten()
		// Should receive an EVENT message
		found := false
		for _, w := range subWritten {
			if msg, ok := w.([]interface{}); ok && len(msg) >= 1 && msg[0] == "EVENT" {
				found = true
				if msg[1] != "sub1" {
					t.Errorf("expected sub_id=sub1, got %v", msg[1])
				}
			}
		}
		if !found {
			t.Error("expected subscriber to receive EVENT broadcast")
		}
	})

	t.Run("maxEvents limit trims old events", func(t *testing.T) {
		r := NewRelay(3, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		r.mu.Lock()
		r.clients[client] = true
		r.mu.Unlock()

		for i := 0; i < 5; i++ {
			evt := makeEvent("pk1", 1, fmt.Sprintf("msg%d", i), int64(1000+i))
			raw, _ := json.Marshal(evt)
			r.handleEvent(client, []json.RawMessage{raw})
		}

		r.mu.RLock()
		if len(r.events) != 3 {
			t.Errorf("expected 3 events after trim, got %d", len(r.events))
		}
		r.mu.RUnlock()
	})
}

func TestHandleReq(t *testing.T) {
	t.Run("REQ with too few args sends Notice", func(t *testing.T) {
		r := NewRelay(100, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		r.handleReq(client, []json.RawMessage{json.RawMessage(`"sub1"`)})

		written := conn.getWritten()
		if len(written) != 1 {
			t.Fatalf("expected 1 message, got %d", len(written))
		}
		msg, ok := written[0].([]string)
		if !ok || msg[0] != "NOTICE" {
			t.Errorf("expected NOTICE, got %v", written[0])
		}
	})

	t.Run("REQ creates subscription and returns matching events from memory", func(t *testing.T) {
		r := NewRelay(100, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		// Pre-populate events
		r.mu.Lock()
		r.events = append(r.events,
			Event{ID: "e1", PubKey: "pk1", Kind: 1, Content: "hello", CreatedAt: 100},
			Event{ID: "e2", PubKey: "pk2", Kind: 2, Content: "world", CreatedAt: 200},
			Event{ID: "e3", PubKey: "pk1", Kind: 1, Content: "foo", CreatedAt: 300},
		)
		r.mu.Unlock()

		subID := `"sub1"`
		filter := `{"kinds":[1]}`
		r.handleReq(client, []json.RawMessage{
			json.RawMessage(subID),
			json.RawMessage(filter),
		})

		// Check subscription created
		client.mu.Lock()
		if _, ok := client.subscriptions["sub1"]; !ok {
			t.Error("expected subscription 'sub1' to be created")
		}
		client.mu.Unlock()

		written := conn.getWritten()
		// Should have 2 EVENT messages + 1 EOSE
		eventCount := 0
		eoseCount := 0
		for _, w := range written {
			if msg, ok := w.([]interface{}); ok && len(msg) >= 1 {
				if msg[0] == "EVENT" {
					eventCount++
				}
			}
			if msg, ok := w.([]string); ok && len(msg) >= 1 && msg[0] == "EOSE" {
				eoseCount++
			}
		}
		if eventCount != 2 {
			t.Errorf("expected 2 EVENT messages (kind=1), got %d", eventCount)
		}
		if eoseCount != 1 {
			t.Errorf("expected 1 EOSE, got %d", eoseCount)
		}
	})

	t.Run("REQ with DB returns events from DB", func(t *testing.T) {
		db := &mockDBProvider{}
		r := NewRelay(100, db)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		// Pre-populate DB
		db.events = []store.NostrEvent{
			{ID: "e1", PubKey: "pk1", Kind: 1, Content: "hello", CreatedAt: 100},
			{ID: "e2", PubKey: "pk1", Kind: 1, Content: "world", CreatedAt: 200},
		}

		subID := `"sub1"`
		filter := `{"kinds":[1],"limit":10}`
		r.handleReq(client, []json.RawMessage{
			json.RawMessage(subID),
			json.RawMessage(filter),
		})

		written := conn.getWritten()
		eventCount := 0
		eoseCount := 0
		for _, w := range written {
			if msg, ok := w.([]interface{}); ok && len(msg) >= 1 && msg[0] == "EVENT" {
				eventCount++
			}
			if msg, ok := w.([]string); ok && len(msg) >= 1 && msg[0] == "EOSE" {
				eoseCount++
			}
		}
		if eventCount != 2 {
			t.Errorf("expected 2 EVENT messages from DB, got %d", eventCount)
		}
		if eoseCount != 1 {
			t.Errorf("expected 1 EOSE, got %d", eoseCount)
		}
	})

	t.Run("REQ respects limit from filter", func(t *testing.T) {
		r := NewRelay(100, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		// Pre-populate 5 events
		r.mu.Lock()
		for i := 0; i < 5; i++ {
			r.events = append(r.events, Event{
				ID: fmt.Sprintf("e%d", i), PubKey: "pk1", Kind: 1,
				Content: fmt.Sprintf("msg%d", i), CreatedAt: int64(100 + i),
			})
		}
		r.mu.Unlock()

		subID := `"sub1"`
		filter := `{"limit":2}`
		r.handleReq(client, []json.RawMessage{
			json.RawMessage(subID),
			json.RawMessage(filter),
		})

		written := conn.getWritten()
		eventCount := 0
		for _, w := range written {
			if msg, ok := w.([]interface{}); ok && len(msg) >= 1 && msg[0] == "EVENT" {
				eventCount++
			}
		}
		if eventCount != 2 {
			t.Errorf("expected 2 EVENT messages (limit=2), got %d", eventCount)
		}
	})
}

func TestHandleClose(t *testing.T) {
	t.Run("removes existing subscription", func(t *testing.T) {
		r := NewRelay(100, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		// Create a subscription
		client.subscriptions["sub1"] = &Subscription{
			ID:     "sub1",
			Filter: Filter{},
			Client: client,
		}

		r.handleClose(client, []json.RawMessage{json.RawMessage(`"sub1"`)})

		client.mu.Lock()
		if _, ok := client.subscriptions["sub1"]; ok {
			t.Error("expected subscription 'sub1' to be removed")
		}
		client.mu.Unlock()
	})

	t.Run("empty raw does nothing", func(t *testing.T) {
		r := NewRelay(100, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		// Should not panic
		r.handleClose(client, []json.RawMessage{})
	})

	t.Run("close non-existent subscription is safe", func(t *testing.T) {
		r := NewRelay(100, nil)
		conn := newMockConn()
		client := &Client{conn: conn, subscriptions: make(map[string]*Subscription)}

		// Should not panic
		r.handleClose(client, []json.RawMessage{json.RawMessage(`"nonexistent"`)})
	})
}
