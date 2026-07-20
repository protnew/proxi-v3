package chat

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
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
	SendChannelSize = 1024
)

// Client represents a connected WebSocket user.
type Client struct {
	UserID      string
	Conn        *websocket.Conn
	Send        chan []byte // outbound messages buffered channel
	hub         *ChatHub
	cancel      context.CancelFunc
	closeOnce   sync.Once
	closeCode   websocket.StatusCode
	closeReason string
}

func (c *Client) forceClose(code websocket.StatusCode, reason string) {
	c.closeOnce.Do(func() {
		c.closeCode = code
		c.closeReason = reason
		// Write the close frame asynchronously. ReadPump will continue to read
		// until the peer echoes the close frame, at which point Read will return an error.
		go c.Conn.Close(code, reason)
	})
}

// ChatHub maintains the set of active clients and broadcasts messages.
type ChatHub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]bool // P6 FIXED: userId → set of *Client (multi-device support)

	// OnMessage is called for every incoming chat message.
	// If nil, messages are only routed internally.
	OnMessage func(msg *Message)

	workerPool chan *Message
}

// NewChatHub creates a new ChatHub.
func NewChatHub() *ChatHub {
	h := &ChatHub{
		clients:    make(map[string]map[*Client]bool),
		workerPool: make(chan *Message, 1024),
	}

	// Start workers for OnMessage processing
	for i := 0; i < 10; i++ {
		go func() {
			for msg := range h.workerPool {
				if h.OnMessage != nil {
					h.OnMessage(msg)
				}
			}
		}()
	}
	return h
}

// Register adds a client to the hub. If a client with the same userID already
// exists the old connection is replaced (the old client's Send channel is closed).
func (h *ChatHub) Register(c *Client) {
	// Encode messages outside of the lock
	welcome, _ := (&Message{
		Type: TypeJoin,
		From: "system",
		To:   c.UserID,
		Text: "connected",
		Ts:   time.Now().Unix(),
	}).Encode()

	joinMsg, _ := (&Message{
		Type: TypeJoin,
		From: c.UserID,
		To:   BroadcastTarget,
		Ts:   time.Now().Unix(),
	}).Encode()

	h.mu.Lock()
	defer h.mu.Unlock()

	// P6 FIXED: Multi-device — add client to existing set, don't replace
	if h.clients[c.UserID] == nil {
		h.clients[c.UserID] = make(map[*Client]bool)
	}
	h.clients[c.UserID][c] = true
	c.hub = h

	// Send welcome to the new client directly.
	select {
	case c.Send <- welcome:
	default:
	}

	// Announce join to everyone else.
	h.broadcastLocked(joinMsg, "")
}

// Unregister removes a client from the hub and cleans up.
func (h *ChatHub) Unregister(c *Client) {
	// Encode message outside of the lock
	leaveMsg, _ := (&Message{
		Type: TypeLeave,
		From: c.UserID,
		To:   BroadcastTarget,
		Ts:   time.Now().Unix(),
	}).Encode()

	h.mu.Lock()
	defer h.mu.Unlock()

	// P6 FIXED: Multi-device — remove just this client from the set
	if clients, ok := h.clients[c.UserID]; ok {
		if _, exists := clients[c]; exists {
			delete(clients, c)
			close(c.Send)
			if len(clients) == 0 {
				delete(h.clients, c.UserID)
			}
			h.broadcastLocked(leaveMsg, "")
		}
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
	// P1 FIX: Collect dead clients, close outside lock (no I/O under RLock)
	// P6 FIXED: Multi-device — iterate all clients across all devices
	var dead []*Client
	for uid, clients := range h.clients {
		if uid == excludeUserID {
			continue
		}
		for c := range clients {
			select {
			case c.Send <- data:
			default:
				log.Printf("[chat] send buffer full for user %s, marking for disconnect", uid)
				dead = append(dead, c)
				delete(clients, c)
			}
		}
	}
	for _, c := range dead {
		close(c.Send)
		if c.Conn != nil {
			c.forceClose(websocket.StatusPolicyViolation, "buffer full")
		}
	}
}

// SendTo sends a message to a specific user. Returns false if the user is not
// connected.
func (h *ChatHub) SendTo(userID string, data []byte) bool {
	var deadClients []*Client
	var delivered bool
	
	func() {
		h.mu.RLock()
		defer h.mu.RUnlock()
		
		// P6 FIXED: Multi-device — deliver to all sessions of the user
	clients, ok := h.clients[userID]
		if !ok {
			return
		}
		for c := range clients {
			select {
			case c.Send <- data:
				delivered = true
			default:
				log.Printf("[chat] send buffer full for user %s, marking dead", userID)
				deadClients = append(deadClients, c)
				delete(clients, c)
			}
		}
	}()
	
	// P3 FIX: Close connection outside the lock
	if len(deadClients) > 0 {
		h.mu.Lock()
		if clients, ok := h.clients[userID]; ok && len(clients) == 0 {
			delete(h.clients, userID)
		}
		h.mu.Unlock()
		for _, dc := range deadClients {
			close(dc.Send)
			if dc.Conn != nil {
				dc.forceClose(websocket.StatusPolicyViolation, "buffer full")
			}
		}
	}
	
	return delivered
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
	if c.Conn == nil {
		c.hub.Unregister(c)
		return
	}

	if c.cancel != nil {
		defer c.cancel()
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.WritePump(ctx)
	}()
	go func() {
		defer wg.Done()
		c.pingPump(ctx)
	}()

	c.ReadPump(ctx) // blocks until read-side closes (peer sends close frame or drops)
	
	// Once ReadPump exits, the connection is dead. Cancel the context to stop other pumps.
	if c.cancel != nil {
		c.cancel()
	}
	wg.Wait() // wait for graceful shutdown of WritePump and pingPump

	// If forceClose wasn't called, close with NormalClosure.
	// If forceClose WAS called, go c.Conn.Close already handled it, and calling it again is safe (returns error).
	code := websocket.StatusNormalClosure
	reason := "client disconnected"
	if c.closeCode != 0 {
		code = c.closeCode
		reason = c.closeReason
	}
	c.Conn.Close(code, reason)
}

func (c *Client) pingPump(ctx context.Context) {
	if c.Conn == nil {
		return
	}
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, WriteTimeout)
			err := c.Conn.Ping(polyfillCtxReset(ctx, pingCtx))
			cancel()
			if err != nil {
				c.forceClose(websocket.StatusNormalClosure, "ping failed")
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// ReadPump reads messages from the WebSocket connection and dispatches them
// to the hub. It runs in a goroutine per client.
// It handles both text JSON frames (legacy) and binary voice frames (0x02).
func (c *Client) ReadPump(ctx context.Context) {
	if c.Conn == nil {
		c.hub.Unregister(c)
		return
	}
	
	defer func() {
		c.hub.Unregister(c)
	}()

	c.Conn.SetReadLimit(MaxBinaryVoiceSize)

	// Rate limit: 10 messages per second, burst of 20
	limiter := rate.NewLimiter(rate.Limit(10), 20)

	for {
		msgType, data, err := c.Conn.Read(ctx)
		if err != nil {
			// Normal closure or context cancel — just exit.
			return
		}

		if !limiter.Allow() {
			log.Printf("[chat] rate limit exceeded for user %s, disconnecting", c.UserID)
			c.forceClose(websocket.StatusPolicyViolation, "rate limit exceeded")
			// nhooyr requires us to keep reading until the client echoes the close frame.
			// Read will return a CloseError once the close frame is received.
			continue
		}

		// Handle binary voice frames.
		if msgType == websocket.MessageBinary && IsBinaryVoiceFrame(data) {
			meta, _, err := DecodeBinaryVoice(data)
			if err != nil {
				log.Printf("[chat] invalid binary voice from %s: %v", c.UserID, err)
				continue
			}

			// Build routing message for the OnMessage callback.
			voiceMsg := BinaryVoiceToMessage(meta, c.UserID)

			// Push to worker pool.
			if c.hub.OnMessage != nil {
				select {
				case c.hub.workerPool <- voiceMsg:
				default:
					log.Printf("[chat] worker pool full, dropping OnMessage callback for voice from %s", c.UserID)
				}
			}

			// Route binary frame as-is to other clients.
			switch voiceMsg.To {
			case "", BroadcastTarget:
				c.hub.Broadcast(data, c.UserID)
			default:
				// Direct message — deliver to recipient and echo to self.
				c.hub.SendTo(voiceMsg.To, data)
				c.hub.SendTo(c.UserID, data)
			}
			continue
		}

		// Handle binary stream frames (0x03 prefix).
		if msgType == websocket.MessageBinary && len(data) > 0 && data[0] == BinaryFrameStream {
			// Route stream frame as-is to all other clients
			// (subscribers will filter by stream ID on the client side)
			c.hub.Broadcast(data, c.UserID)

			if c.hub.OnMessage != nil {
				streamMsg := &Message{
					Type: "stream-frame",
					From: c.UserID,
					To:   BroadcastTarget,
					Ts:   time.Now().Unix(),
				}
				select {
				case c.hub.workerPool <- streamMsg:
				default:
				}
			}
			continue
		}

		// Legacy text JSON handling.
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

		// Push to worker pool.
		if c.hub.OnMessage != nil {
			select {
			case c.hub.workerPool <- msg:
			default:
				log.Printf("[chat] worker pool full, dropping OnMessage callback for msg from %s", c.UserID)
			}
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
// It supports both text JSON and binary voice frames.
func (c *Client) WritePump(ctx context.Context) {
	if c.Conn == nil {
		return
	}
	
	// Removed synchronous c.Conn.Close from WritePump to avoid blocking

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				// Channel closed — hub unregistered us.
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, WriteTimeout)

			// Determine frame type: binary voice (0x02) or stream (0x03) or text JSON.
			var writeType websocket.MessageType
			if IsBinaryVoiceFrame(msg) || (len(msg) > 0 && msg[0] == BinaryFrameStream) {
				writeType = websocket.MessageBinary
			} else {
				writeType = websocket.MessageText
			}
			err := c.Conn.Write(writeCtx, writeType, msg)
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
	if conn == nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	client := &Client{
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, SendChannelSize),
		hub:    hub,
		cancel: cancel,
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