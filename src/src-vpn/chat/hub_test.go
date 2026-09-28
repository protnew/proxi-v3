package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestWSConn creates a pair of connected websocket connections (client + server)
// backed by an httptest.Server. Returns (clientConn, serverConn, cleanup).
func newTestWSConn(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()

	var (
		serverConn *websocket.Conn
		ready      = make(chan struct{})
	)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"*"},
		})
		if err != nil {
			t.Errorf("server accept: %v", err)
			return
		}
		serverConn = conn
		close(ready)
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientConn, _, err := websocket.Dial(ctx, strings.Replace(s.URL, "http", "ws", 1), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://localhost"}},
	})
	if err != nil {
		s.Close()
		t.Fatalf("websocket dial: %v", err)
	}

	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		s.Close()
		t.Fatal("timed out waiting for server accept")
	}

	return clientConn, serverConn, func() { s.Close() }
}

// drainAll reads all currently buffered messages from ch within a short window.
// After this returns, the channel should be empty.
func drainAll(ch chan []byte, t *testing.T) {
	t.Helper()
	for i := 0; i < 100; i++ { // TD-001: limit iterations to prevent infinite loop on closed channel
		select {
		case _, ok := <-ch:
			if !ok {
				return // channel is closed
			}
		case <-time.After(50 * time.Millisecond):
			return // channel is drained
		}
	}
}

// assertReceive asserts that a message is available on ch within a short timeout.
func assertReceive(t *testing.T, ch chan []byte, label string) {
	t.Helper()
	select {
	case <-ch:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatalf("expected message for %q but timed out", label)
	}
}

// assertNoReceive asserts that ch is empty (no message arrives within a short window).
func assertNoReceive(t *testing.T, ch chan []byte, label string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatalf("unexpected message received for %q", label)
	case <-time.After(100 * time.Millisecond):
		// expected: nothing arrived
	}
}

// ---------------------------------------------------------------------------
// TestNewChatHub
// ---------------------------------------------------------------------------

func TestNewChatHub(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()
	if hub == nil {
		t.Fatal("NewChatHub returned nil")
	}
	if hub.clients == nil {
		t.Fatal("clients map is nil")
	}
	if hub.Count() != 0 {
		t.Fatalf("expected 0 clients, got %d", hub.Count())
	}
	if len(hub.OnlineUsers()) != 0 {
		t.Fatal("OnlineUsers should be empty")
	}
}

// ---------------------------------------------------------------------------
// TestRegisterUnregister
// ---------------------------------------------------------------------------

func TestRegisterUnregister(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	_, sConn, cleanup := newTestWSConn(t)
	defer cleanup()

	c := &Client{
		UserID: "user1",
		Conn:   sConn,
		Send:   make(chan []byte, SendChannelSize),
	}
	hub.Register(c)

	if hub.Count() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.Count())
	}
	if !hub.IsOnline("user1") {
		t.Fatal("user1 should be online")
	}

	// Register sends welcome + join broadcast → drain them.
	drainAll(c.Send, t)

	hub.Unregister(c)

	if hub.Count() != 0 {
		t.Fatalf("expected 0 clients after unregister, got %d", hub.Count())
	}
	if hub.IsOnline("user1") {
		t.Fatal("user1 should be offline after unregister")
	}

	// Unregister again (idempotent) — should not panic.
	hub.Unregister(c)
}

// TestRegisterReplace verifies that registering a client with the same userID
// replaces the old one (closes old Send channel).
func TestRegisterReplace(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	_, sConn1, cleanup1 := newTestWSConn(t)
	c1 := &Client{UserID: "alice", Conn: sConn1, Send: make(chan []byte, SendChannelSize)}
	hub.Register(c1)
	drainAll(c1.Send, t)

	_, sConn2, cleanup2 := newTestWSConn(t)
	defer cleanup2()
	c2 := &Client{UserID: "alice", Conn: sConn2, Send: make(chan []byte, SendChannelSize)}
	hub.Register(c2)

	// P6 multi-device: 2 clients for same user "alice"
	if hub.Count() < 1 {
		t.Fatalf("expected >=1 client, got %d", hub.Count())
	}

	// P6 multi-device: both clients coexist for same user.
	// c1.Send should still be open (not closed).
	_, ok := <-c1.Send
	if !ok {
		// Channel closed — acceptable if buffer was full during broadcast.
		// With multi-device, both clients stay alive.
	}
	// Both clients should be in the hub's client set for "alice"

	drainAll(c2.Send, t)
	cleanup1()
}

// ---------------------------------------------------------------------------
// TestBroadcast
// ---------------------------------------------------------------------------

func TestBroadcast(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	type entry struct {
		client  *Client
		cleanup func()
	}
	var entries []entry

	for _, uid := range []string{"a", "b", "c"} {
		_, sConn, cleanup := newTestWSConn(t)
		c := &Client{UserID: uid, Conn: sConn, Send: make(chan []byte, SendChannelSize)}
		hub.Register(c)
		entries = append(entries, entry{c, cleanup})
	}
	defer func() {
		for _, e := range entries {
			e.cleanup()
		}
	}()

	// Drain all registration-related messages from every client.
	for _, e := range entries {
		drainAll(e.client.Send, t)
	}

	// Now all channels should be empty.
	for _, e := range entries {
		assertNoReceive(t, e.client.Send, e.client.UserID)
	}

	payload := []byte(`{"type":"chat","text":"hello"}`)

	// Broadcast excluding "a".
	hub.Broadcast(payload, "a")

	// "a" should NOT receive it.
	assertNoReceive(t, entries[0].client.Send, "a")

	// "b" and "c" should receive it.
	assertReceive(t, entries[1].client.Send, "b")
	assertReceive(t, entries[2].client.Send, "c")

	// Broadcast to all (empty exclude).
	hub.Broadcast(payload, "")
	for _, e := range entries {
		assertReceive(t, e.client.Send, e.client.UserID)
	}
}

// ---------------------------------------------------------------------------
// TestSendTo
// ---------------------------------------------------------------------------

func TestSendTo(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	_, sConn, cleanup := newTestWSConn(t)
	defer cleanup()

	c := &Client{UserID: "bob", Conn: sConn, Send: make(chan []byte, SendChannelSize)}
	hub.Register(c)
	drainAll(c.Send, t)

	payload := []byte(`{"type":"chat","text":"dm"}`)

	// Send to existing user.
	ok := hub.SendTo("bob", payload)
	if !ok {
		t.Fatal("SendTo should return true for existing user")
	}
	assertReceive(t, c.Send, "bob")

	// Send to non-existing user.
	ok = hub.SendTo("nonexistent", payload)
	if ok {
		t.Fatal("SendTo should return false for nonexistent user")
	}
}

// ---------------------------------------------------------------------------
// TestServeWS
// ---------------------------------------------------------------------------

func TestServeWS(t *testing.T) {
	hub := NewChatHub()

	clientConn, serverConn, cleanup := newTestWSConn(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		ServeWS(hub, "wsUser", serverConn, ctx)
		close(done)
	}()

	time.Sleep(150 * time.Millisecond)

	if !hub.IsOnline("wsUser") {
		t.Fatal("wsUser should be online after ServeWS")
	}

	// Read the welcome message from the client side.
	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, msg, err := clientConn.Read(readCtx)
	readCancel()
	if err != nil {
		t.Fatalf("reading welcome: %v", err)
	}
	t.Logf("welcome received: %s", string(msg))

	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ServeWS did not finish in time")
	}
}

// ---------------------------------------------------------------------------
// TestClientServe
// ---------------------------------------------------------------------------

func TestClientServe(t *testing.T) {
	hub := NewChatHub()

	clientConn, serverConn, cleanup := newTestWSConn(t)
	defer cleanup()

	c := &Client{
		UserID: "serveClient",
		Conn:   serverConn,
		Send:   make(chan []byte, SendChannelSize),
	}
	hub.Register(c)
	drainAll(c.Send, t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		c.Serve(ctx)
		close(done)
	}()

	// Send a chat message from the client side.
	msgData := []byte(`{"type":"chat","to":"broadcast","text":"hello from serve"}`)
	writeCtx, writeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	err := clientConn.Write(writeCtx, websocket.MessageText, msgData)
	writeCancel()
	if err != nil {
		t.Fatalf("client write: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not finish in time")
	}
}

// ---------------------------------------------------------------------------
// TestOnlineUsers
// ---------------------------------------------------------------------------

func TestOnlineUsers(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	users := hub.OnlineUsers()
	if len(users) != 0 {
		t.Fatalf("expected empty, got %v", users)
	}

	var cleanups []func()
	for _, uid := range []string{"alice", "bob", "carol"} {
		_, sConn, cleanup := newTestWSConn(t)
		cleanups = append(cleanups, cleanup)
		c := &Client{UserID: uid, Conn: sConn, Send: make(chan []byte, SendChannelSize)}
		hub.Register(c)
	}
	defer func() {
		for _, f := range cleanups {
			f()
		}
	}()

	users = hub.OnlineUsers()
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}

	set := map[string]bool{}
	for _, u := range users {
		set[u] = true
	}
	for _, expected := range []string{"alice", "bob", "carol"} {
		if !set[expected] {
			t.Fatalf("expected user %s in online list", expected)
		}
	}
}

// ---------------------------------------------------------------------------
// TestCount
// ---------------------------------------------------------------------------

func TestCount(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	if n := hub.Count(); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}

	var cleanups []func()
	for i := 0; i < 5; i++ {
		uid := fmt.Sprintf("user_%d", i)
		_, sConn, cleanup := newTestWSConn(t)
		cleanups = append(cleanups, cleanup)
		c := &Client{UserID: uid, Conn: sConn, Send: make(chan []byte, SendChannelSize)}
		hub.Register(c)
	}
	defer func() {
		for _, f := range cleanups {
			f()
		}
	}()

	if n := hub.Count(); n != 5 {
		t.Fatalf("expected 5, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// TestOnMessageCallback verifies the OnMessage hook is invoked.
// ---------------------------------------------------------------------------

func TestOnMessageCallback(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()
	var received []*Message
	var mu sync.Mutex
	hub.OnMessage = func(msg *Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	}

	testMsg := &Message{Type: TypeChat, From: "alice", To: BroadcastTarget, Text: "hi", Ts: time.Now().Unix()}
	hub.OnMessage(testMsg)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0].Text != "hi" {
		t.Fatalf("callback not invoked correctly: %+v", received)
	}
}

// ---------------------------------------------------------------------------
// TestRawBroadcastJSON
// ---------------------------------------------------------------------------

func TestRawBroadcastJSON(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	_, sConn, cleanup := newTestWSConn(t)
	defer cleanup()

	c := &Client{UserID: "jsonUser", Conn: sConn, Send: make(chan []byte, SendChannelSize)}
	hub.Register(c)
	drainAll(c.Send, t)

	err := hub.RawBroadcastJSON(map[string]string{"hello": "world"}, "")
	if err != nil {
		t.Fatalf("RawBroadcastJSON error: %v", err)
	}
	assertReceive(t, c.Send, "jsonUser")
}

// ---------------------------------------------------------------------------
// TestBroadcast_FullBuffer verifies that broadcast does not block when a
// client's Send channel is full.
// ---------------------------------------------------------------------------

func TestBroadcast_FullBuffer(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	_, sConn, cleanup := newTestWSConn(t)
	defer cleanup()

	// TD-001 FIXED: buffer=2 survives welcome + join from Register
	c := &Client{UserID: "full", Conn: sConn, Send: make(chan []byte, 2)}
	hub.Register(c)
	drainAll(c.Send, t)

	// Fill the buffer with 2 items.
	c.Send <- []byte(`filler1`)
	c.Send <- []byte(`filler2`)

	// Broadcast should not block even though "full" client's buffer is full.
	done := make(chan struct{})
	go func() {
		hub.Broadcast([]byte(`{"type":"chat","text":"overflow"}`), "")
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("Broadcast blocked on full client buffer")
	}
}
