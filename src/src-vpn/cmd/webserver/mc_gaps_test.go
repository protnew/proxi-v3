package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/store"
	"nhooyr.io/websocket"
)

func TestMC4_BlindLogOmitsText(t *testing.T) {
	msg := &chat.Message{Type: chat.TypeChat, Text: "super-secret-body", Encrypted: true}
	line := blindLogLine(msg)
	if strings.Contains(line, "super-secret-body") {
		t.Fatal(line)
	}
}

func TestMC3_MC4_SaveTwiceAndCipher(t *testing.T) {
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	msg := &chat.Message{Type: chat.TypeChat, ID: "same-id", From: "alice", To: "bob", Text: "nip44:aabbcc", Ts: 10, Encrypted: true}
	if err := saveIncomingChat(db, msg); err != nil {
		t.Fatal(err)
	}
	if err := saveIncomingChat(db, msg); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM messages WHERE id = 'same-id'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows=%d", n)
	}
	var raw string
	var enc int
	if err := db.DB().QueryRow(`SELECT text, encrypted FROM messages WHERE id = 'same-id'`).Scan(&raw, &enc); err != nil {
		t.Fatal(err)
	}
	if enc != 1 || strings.Contains(raw, "hello-plain") || !strings.HasPrefix(raw, "enc2:") {
		t.Fatalf("raw=%q enc=%d", raw, enc)
	}
	if strings.Contains(raw, "nip44:aabbcc") {
		t.Fatal("e2e blob stored without at-rest wrap")
	}
	plain := &chat.Message{Type: chat.TypeChat, ID: "nope", From: "a", To: "b", Text: "hello-plain", Encrypted: true, Ts: 11}
	if err := saveIncomingChat(db, plain); err == nil {
		t.Fatal("plaintext under encrypted must fail")
	}
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM messages WHERE id = 'nope'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("refused row stored")
	}
}

func TestMC3_WSDuplicate(t *testing.T) {
	db, err := store.NewStore(filepath.Join(t.TempDir(), "mc3.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hub := chat.NewChatHub()
	done := make(chan struct{}, 4)
	hub.OnMessage = func(msg *chat.Message) {
		_ = saveIncomingChat(db, msg)
		done <- struct{}{}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		chat.ServeWS(hub, "alice", conn, r.Context())
	}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+s.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	body, _ := json.Marshal(&chat.Message{Type: chat.TypeChat, ID: "net-1", To: "bob", Text: "nip44:net", Ts: 20})
	for i := 0; i < 2; i++ {
		if err := conn.Write(ctx, websocket.MessageText, body); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("persist did not run")
		}
	}
	var n int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM messages WHERE id = 'net-1'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("network rows=%d", n)
	}
}
