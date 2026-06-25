package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestClient_NilConn(t *testing.T) {
	hub := NewChatHub()
	
	// Ensure no panic when Conn is nil
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic occurred when Conn is nil: %v", r)
		}
	}()

	client := &Client{
		UserID: "test_user_nil",
		Conn:   nil,
		Send:   make(chan []byte, 10), // Small buffer
		hub:    hub,
	}

	// Should safely return or handle nil Conn without panic
	client.Serve(context.Background())
	client.ReadPump(context.Background())
	client.WritePump(context.Background())

	// Test Register with nil Conn
	hub.Register(client)
	
	// Test unregister
	hub.Unregister(client)

	// Re-register to test broadcast and SendTo on nil Conn
	client2 := &Client{
		UserID: "test_user_nil_2",
		Conn:   nil,
		Send:   make(chan []byte, 10),
		hub:    hub,
	}
	hub.Register(client2)

	// Fill buffer to trigger policy violation drop
	for i := 0; i < 20; i++ {
		hub.SendTo("test_user_nil_2", []byte("test"))
	}
	
	// Test ServeWS with nil
	ServeWS(hub, "test2_nil", nil, context.Background())
}

func TestClient_WebSocketConnection(t *testing.T) {
	hub := NewChatHub()
	
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Logf("websocket accept: %v", err)
			return
		}
		
		ServeWS(hub, "ws_user", conn, r.Context())
	}))
	defer s.Close()

	wsURL := "ws" + s.URL[4:]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusInternalError, "test done")

	// Verify the user gets registered
	time.Sleep(200 * time.Millisecond)
	if !hub.IsOnline("ws_user") {
		t.Errorf("expected ws_user to be online")
	}

	// Send a message
	err = conn.Write(ctx, websocket.MessageText, []byte(`{"to":"broadcast","text":"hello"}`))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = conn.Close(websocket.StatusNormalClosure, "")
	if err != nil {
		t.Logf("close err: %v", err)
	}
	
	// Wait for unregister
	time.Sleep(200 * time.Millisecond)
	if hub.IsOnline("ws_user") {
		t.Errorf("expected ws_user to be offline after close")
	}
}
