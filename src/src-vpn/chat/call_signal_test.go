package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func callServer(t *testing.T, hub *ChatHub) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		ServeWS(hub, r.URL.Query().Get("u"), conn, r.Context())
	}))
}

func dialUser(t *testing.T, s *httptest.Server, user string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, "ws"+s.URL[4:]+"?u="+user, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })
	return conn
}

func readUntil(t *testing.T, conn *websocket.Conn, want string, timeout time.Duration) Message {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		_, data, err := conn.Read(ctx)
		cancel()
		if err != nil {
			t.Fatalf("read %s: %v", want, err)
		}
		var m Message
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		if m.Type == want {
			return m
		}
	}
	t.Fatalf("timeout waiting %s", want)
	return Message{}
}

func TestMC2_KeyExchangeRings(t *testing.T) {
	hub := NewChatHub()
	s := callServer(t, hub)
	defer s.Close()
	alice := dialUser(t, s, "alice")
	bob := dialUser(t, s, "bob")
	time.Sleep(150 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, _ := json.Marshal(&Message{Type: TypeKeyExchange, To: "bob", PublicKey: "pk", ID: "call-1"})
	if err := alice.Write(ctx, websocket.MessageText, body); err != nil {
		t.Fatal(err)
	}
	offer := readUntil(t, bob, TypeKeyExchange, 2*time.Second)
	if offer.PublicKey != "pk" || offer.From != "alice" {
		t.Fatalf("%+v", offer)
	}
	ring := readUntil(t, alice, "ringing", 2*time.Second)
	if ring.From != "bob" {
		t.Fatalf("%+v", ring)
	}
	ans, _ := json.Marshal(&Message{Type: "call-answer", To: "alice", Text: "sdp-answer", ID: "call-1"})
	if err := bob.Write(ctx, websocket.MessageText, ans); err != nil {
		t.Fatal(err)
	}
	got := readUntil(t, alice, "call-answer", 2*time.Second)
	if got.Text != "sdp-answer" {
		t.Fatal(got.Text)
	}
}

func TestMC2_OfflineDoesNotHang(t *testing.T) {
	hub := NewChatHub()
	s := callServer(t, hub)
	defer s.Close()
	alice := dialUser(t, s, "alice")
	time.Sleep(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, _ := json.Marshal(&Message{Type: TypeKeyExchange, To: "missing", PublicKey: "pk"})
	if err := alice.Write(ctx, websocket.MessageText, body); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	got := readUntil(t, alice, "call-error", 2*time.Second)
	if time.Since(start) > 1500*time.Millisecond {
		t.Fatal("hung", time.Since(start))
	}
	if got.Type != "call-error" {
		t.Fatal(got.Type)
	}
}

func TestMC2_HubCloseErrors(t *testing.T) {
	hub := NewChatHub()
	s := callServer(t, hub)
	alice := dialUser(t, s, "alice")
	time.Sleep(100 * time.Millisecond)
	hub.MarkDown()
	s.Close()
	deadline := time.Now().Add(2 * time.Second)
	sawErr := false
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		_, _, err := alice.Read(ctx)
		cancel()
		if err != nil {
			sawErr = true
			break
		}
	}
	if !sawErr {
		t.Fatal("expected error after hub close")
	}
}
