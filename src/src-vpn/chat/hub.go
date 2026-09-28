package chat

import (
	"context"
	"log"
	"nhooyr.io/websocket"
	"sync"
	"time"
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
	down       bool
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
	h.mu.Lock()
	defer h.mu.Unlock()
	h.broadcastLocked(data, excludeUserID)
}

// broadcastLocked is the internal broadcast helper. Caller must hold the write-lock
// because a full send buffer deletes the client from the map.
func (h *ChatHub) broadcastLocked(data []byte, excludeUserID string) {
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
				if len(clients) == 0 {
					delete(h.clients, uid)
				}
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

	h.mu.Lock()
	clients, ok := h.clients[userID]
	if ok {
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
		if len(clients) == 0 {
			delete(h.clients, userID)
		}
	}
	h.mu.Unlock()

	for _, dc := range deadClients {
		close(dc.Send)
		if dc.Conn != nil {
			dc.forceClose(websocket.StatusPolicyViolation, "buffer full")
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
