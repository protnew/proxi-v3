// File: hub_pumps.go
// Split from hub.go: Serve/ReadPump/WritePump/pingPump/ServeWS.

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


// Client represents a connected WebSocket user.

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

		// P5: refuse chat frames that claim E2E without ciphertext (no plaintext under encrypted:true).
		if msg.Type == TypeChat {
			claimed := msg.ClaimedEncrypted()
			if claimed && !LooksLikeClientCiphertext(msg.Text) {
				log.Printf("[chat] P5 refuse: encrypted flag without ciphertext from %s", c.UserID)
				continue
			}
			msg.NormalizeE2EFlags()
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
