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

// TestSpammerE2E simulates multiple spammers sending messages as fast as possible (20+ messages per second),
// verifies that each client is disconnected with a 1008 Policy Violation, and ensures that the server does not crash.
func TestSpammerE2E(t *testing.T) {
	hub := NewChatHub()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		
		uid := r.URL.Query().Get("uid")
		if uid == "" {
			uid = "spammer"
		}
		
		ServeWS(hub, uid, conn, r.Context())
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Start concurrent spammers
	var wg sync.WaitGroup
	numSpammers := 50

	for i := 0; i < numSpammers; i++ {
		wg.Add(1)
		go func(spammerID int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			url := fmt.Sprintf("%s?uid=spammer_%d", wsURL, spammerID)
			conn, _, err := websocket.Dial(ctx, url, nil)
			if err != nil {
				// Server might be overwhelmed, which is acceptable in stress tests, but ideally they all connect
				t.Logf("Spammer %d failed to connect: %v", spammerID, err)
				return
			}

			// We need to read from the connection to capture the CloseError
			var closeErr error
			readDone := make(chan struct{})
			go func() {
				for {
					_, _, err := conn.Read(ctx)
					if err != nil {
						closeErr = err
						close(readDone)
						return
					}
				}
			}()

			// Send messages as fast as possible to bypass the 10 msgs/sec and 20 burst limit.
			// Sending 50 messages should definitively trigger the 1008 Policy Violation.
			for j := 0; j < 50; j++ {
				err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"chat","text":"spam"}`))
				if err != nil {
					break
				}
				// Small sleep to ensure we actually hit 20+ messages/sec but still fast enough to be spam
				time.Sleep(10 * time.Millisecond)
			}

			<-readDone

			if closeErr == nil {
				t.Errorf("Spammer %d expected connection to be closed due to rate limit, but it wasn't", spammerID)
				return
			}

			cerr := websocket.CloseStatus(closeErr)
			if cerr != websocket.StatusPolicyViolation {
				t.Errorf("Spammer %d expected close code %v (PolicyViolation), got %v (err: %v)", spammerID, websocket.StatusPolicyViolation, cerr, closeErr)
			}
		}(i)
	}

	wg.Wait()
	
	// Ensure server is still alive and accepting normal connections
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"?uid=normal_user", nil)
	if err != nil {
		t.Fatalf("Server crash check: failed to connect normal user after spam attack: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	
	// Wait a bit for all spammers to unregister and normal_user to register.
	// websocket.Dial returns when the handshake completes, which might be before hub.Register executes on the server.
	var online []string
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		online = hub.OnlineUsers()
		if len(online) == 1 && online[0] == "normal_user" {
			break
		}
	}

	if len(online) != 1 {
		t.Errorf("Expected 1 online user (the normal one), got %d: %v", len(online), online)
	}
}
