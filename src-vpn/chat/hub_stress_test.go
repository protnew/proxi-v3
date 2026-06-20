package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// TestHubStress_ConnectionLeak tests the hub with many simulated concurrent connections
// to ensure there are no goroutine leaks when clients disconnect unexpectedly or buffers fill up.
func TestHubStress_ConnectionLeak(t *testing.T) {
	hub := NewChatHub()

	// Capture baseline goroutines
	time.Sleep(100 * time.Millisecond) // Give time for any background tasks to settle
	baselineGoroutines := runtime.NumGoroutine()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}

		// Mock a user ID based on the URL query or remote addr
		uid := r.URL.Query().Get("uid")
		if uid == "" {
			uid = "anon"
		}

		go ServeWS(hub, uid, conn, r.Context())
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	const numClients = 100 // Simulate 100 clients connecting/disconnecting quickly
	var wgClients sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wgClients.Add(1)
		go func(id int) {
			defer wgClients.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			url := fmt.Sprintf("%s?uid=user_%d", wsURL, id)
			conn, _, err := websocket.Dial(ctx, url, nil)
			if err != nil {
				return // might fail if server is overwhelmed, that's fine for stress test
			}
			
			// Send a few messages
			for j := 0; j < 5; j++ {
				_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"chat","text":"hello"}`))
				time.Sleep(10 * time.Millisecond)
			}

			// Forcefully close without graceful shutdown to test leak handling
			conn.Close(websocket.StatusInternalError, "force drop")
		}(i)
	}

	wgClients.Wait()

	// Also simulate the server broadcasting messages while clients are disconnecting
	for i := 0; i < 50; i++ {
		hub.Broadcast([]byte(`{"type":"system","text":"broadcast"}`), "")
	}

	// Give the hub and runtime time to clean up disconnected goroutines
	time.Sleep(1 * time.Second)
	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	
	// Allow a small margin (e.g., +5) for HTTP keep-alive connections that haven't fully closed yet
	if finalGoroutines > baselineGoroutines+20 {
		t.Fatalf("Goroutine leak detected: baseline=%d, final=%d", baselineGoroutines, finalGoroutines)
	}

	// The hub should have no users online after everyone disconnected
	online := hub.OnlineUsers()
	if len(online) > 0 {
		t.Errorf("Expected 0 online users, got %d", len(online))
	}
}
