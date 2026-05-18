package chat

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

const (
	// MaxMessageSize is the maximum size of a single message (4 KB).
	MaxMessageSize = 4096

	// PingInterval is how often the server sends pings to clients.
	PingInterval = 30 * time.Second

	// WriteTimeout is the maximum time to wait when writing to a client.
	WriteTimeout = 10 * time.Second

	// SendChannelSize is the buffer size for the per-client send channel.
	SendChannelSize = 64
)

// Client represents a connected WebSocket user.
type Client struct {
	UserID string
	Conn   *websocket.Conn
	Send   chan []byte // outbound messages buffered channel
	hub    *ChatHub
}

// ChatHub maintains the set of active clients and broadcasts messages.
type ChatHub struct {
	mu      sync.RWMutex
	clients map[string]*Client // userId → *Client

	// OnMessage is called for every incoming chat message.
	// If nil, messages are only routed internally.
	OnMessage func(msg *Message)
}

// NewChatHub creates a new ChatHub.
func NewChatHub() *ChatHub {
	return &ChatHub{
		clients: make(map[string]*Client),
	}
}

// Register adds a client to the hub. If a client with the same userID already
// exists the old connection is replaced (the old client's Send channel is closed).
func (h *ChatHub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if old, ok := h.clients[c.UserID]; ok {
		close(old.Send)
		_ = old.Conn.Close(websocket.StatusNormalClosure, "replaced by new connection")
	}

	c.hub = h
	h.clients[c.UserID] = c

	// Send welcome to the new client directly.
	welcome, _ := (&Message{
		Type: TypeJoin,
		From: "system",
		To:   c.UserID,
		Text: "connected",
		Ts:   time.Now().Unix(),
	}).Encode()
	select {
	case c.Send <- welcome:
	default:
	}

	// Announce join to everyone else.
	joinMsg, _ := (&Message{
		Type: TypeJoin,
		From: c.UserID,
		To:   BroadcastTarget,
		Ts:   time.Now().Unix(),
	}).Encode()
	h.broadcastLocked(joinMsg, "")
}

// Unregister removes a client from the hub and cleans up.
func (h *ChatHub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.clients[c.UserID]; ok && existing == c {
		delete(h.clients, c.UserID)
		close(c.Send)

		// Announce leave.
		leaveMsg, _ := (&Message{
			Type: TypeLeave,
			From: c.UserID,
			To:   BroadcastTarget,
			Ts:   time.Now().Unix(),
		}).Encode()
		h.broadcastLocked(leaveMsg, "")
	}
}

// Broadcast sends a message to every connected client except the sender
// (identified by excludeUserID, empty = send to all).
func (h *ChatHub) Broadcast(data []byte, excludeUserID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	h.broadcastLocked(data, excludeUserID)
}

// broadcastLocked is the internal broadcast helper. Caller must hold at least
// a read-lock on h.mu.
func (h *ChatHub) broadcastLocked(data []byte, excludeUserID string) {
	for uid, c := range h.clients {
		if uid == excludeUserID {
			continue
		}
		select {
		case c.Send <- data:
		default:
			// Client buffer full — best-effort drop; the WritePump will detect
			// and disconnect.
			log.Printf("[chat] send buffer full for user %s, dropping message", uid)
		}
	}
}

// SendTo sends a message to a specific user. Returns false if the user is not
// connected.
func (h *ChatHub) SendTo(userID string, data []byte) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	c, ok := h.clients[userID]
	if !ok {
		return false
	}
	select {
	case c.Send <- data:
		return true
	default:
		log.Printf("[chat] send buffer full for user %s, dropping direct message", userID)
		return true // user is connected, just slow
	}
}

// OnlineUsers returns the list of currently connected user IDs.
func (h *ChatHub) OnlineUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := make([]string, 0, len(h.clients))
	for uid := range h.clients {
		users = append(users, uid)
	}
	return users
}

// IsOnline checks whether a given user is connected.
func (h *ChatHub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

// Count returns the number of connected clients.
func (h *ChatHub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// --- Client read / write pumps ------------------------------------------------

// Serve starts the ReadPump and WritePump goroutines for a client.
// It blocks until both pumps have finished, which means the client has
// disconnected.
func (c *Client) Serve(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.WritePump(ctx)
	c.ReadPump(ctx) // blocks until read-side closes
}

// ReadPump reads messages from the WebSocket connection and dispatches them
// to the hub. It runs in a goroutine per client.
func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c)
		_ = c.Conn.Close(websocket.StatusNormalClosure, "read pump done")
	}()

	c.Conn.SetReadLimit(MaxMessageSize)

	for {
		_, data, err := c.Conn.Read(ctx)
		if err != nil {
			// Normal closure or context cancel — just exit.
			return
		}

		msg, err := DecodeMessage(data)
		if err != nil {
			log.Printf("[chat] invalid message from %s: %v", c.UserID, err)
			continue
		}

		// Stamp the sender.
		msg.From = c.UserID
		if msg.Ts == 0 {
			msg.Ts = time.Now().Unix()
		}

		// Invoke callback if set.
		if c.hub.OnMessage != nil {
			c.hub.OnMessage(msg)
		}

		// Route message.
		encoded, _ := msg.Encode()

		switch msg.To {
		case "", BroadcastTarget:
			c.hub.Broadcast(encoded, c.UserID)
		default:
			// Direct message — deliver to recipient and echo back to sender.
			c.hub.SendTo(msg.To, encoded)
			// Also deliver echo to self for multi-device sync.
			c.hub.SendTo(c.UserID, encoded)
		}
	}
}

// WritePump writes buffered messages to the WebSocket connection.
// It also handles periodic ping/pong keepalive.
func (c *Client) WritePump(ctx context.Context) {
	ticker := time.NewTicker(PingInterval)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close(websocket.StatusNormalClosure, "write pump done")
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				// Channel closed — hub unregistered us.
				_ = c.Conn.Close(websocket.StatusNormalClosure, "hub unregistered")
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, WriteTimeout)
			err := c.Conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}

		case <-ticker.C:
			// Send ping.
			pingCtx, cancel := context.WithTimeout(ctx, WriteTimeout)
			err := c.Conn.Ping(polyfillCtxReset(ctx, pingCtx))
			cancel()
			if err != nil {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// polyfillCtxReset is a helper that returns a context suitable for sending a
// ping. nhooyr.io/websocket Ping accepts a context.
func polyfillCtxReset(_ context.Context, timeoutCtx context.Context) context.Context {
	return timeoutCtx
}

// ServeWS upgrades an HTTP connection to WebSocket and begins serving the
// client. This is a convenience function for HTTP handlers.
//
// Example usage in an HTTP handler:
//
//	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
//	…
//	client := &chat.Client{UserID: userID, Conn: conn, Send: make(chan []byte, chat.SendChannelSize)}
//	hub.Register(client)
//	client.Serve(r.Context())
func ServeWS(hub *ChatHub, userID string, conn *websocket.Conn, ctx context.Context) {
	client := &Client{
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, SendChannelSize),
		hub:    hub,
	}
	hub.Register(client)
	client.Serve(ctx)
}

// RawBroadcastJSON is a helper that broadcasts an arbitrary JSON-serializable
// value to all connected clients except the sender.
func (h *ChatHub) RawBroadcastJSON(v interface{}, excludeUserID string) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	h.Broadcast(data, excludeUserID)
	return nil
}
