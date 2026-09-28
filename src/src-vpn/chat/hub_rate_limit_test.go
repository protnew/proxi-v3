package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestHubRateLimit(t *testing.T) {
	hub := NewChatHub()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		ServeWS(hub, "testuser", conn, r.Context())
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// The limiter is 10 msgs/sec with a burst of 20.
	// We will send 25 messages immediately.
	// The 21st or 22nd should trigger PolicyViolation.
	var closeErr error
	
	// Start a goroutine to read until error
	done := make(chan struct{})
	go func() {
		for {
			_, _, err := conn.Read(ctx)
			if err != nil {
				closeErr = err
				close(done)
				return
			}
		}
	}()

	for i := 0; i < 25; i++ {
		err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"chat","text":"hello"}`))
		if err != nil {
			break
		}
	}

	<-done

	if closeErr == nil {
		t.Fatalf("Expected connection to be closed due to rate limit, but it wasn't")
	}

	// We expect a CloseError with status PolicyViolation (1008)
	cerr := websocket.CloseStatus(closeErr)
	if cerr != websocket.StatusPolicyViolation {
		t.Fatalf("Expected close code %v (PolicyViolation), got %v (err: %v)", websocket.StatusPolicyViolation, cerr, closeErr)
	}
	
	// Note: nhooyr.io/websocket doesn't expose the reason easily via CloseStatus.
	// But AsCloseError lets us inspect it.
}
