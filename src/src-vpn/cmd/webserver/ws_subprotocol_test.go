package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestWS_SubprotocolTokenAccepted(t *testing.T) {
	srv, authSvc, _ := setupAuthServer(t)
	tok, _, err := authSvc.GenerateTokenPair("user-1", "npub1subproto")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"proxi", "proxi-jwt." + tok},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	if resp.StatusCode != 101 {
		t.Fatalf("status=%d want 101", resp.StatusCode)
	}
	echo := resp.Header.Get("Sec-WebSocket-Protocol")
	if echo != "proxi" {
		t.Fatalf("echo=%q want proxi", echo)
	}
	if strings.Contains(echo, tok) || strings.Contains(resp.Header.Get("Sec-WebSocket-Protocol"), "proxi-jwt") {
		t.Fatal("jwt leaked in echoed subprotocol")
	}
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(buf.String(), "WS connected: npub1subproto") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(buf.String(), "WS connected: npub1subproto") {
		t.Fatalf("claims not applied, log=%s", buf.String())
	}
	if strings.Contains(buf.String(), tok) {
		t.Fatal("token leaked into log")
	}
}

func TestWS_QueryTokenDeprecated(t *testing.T) {
	srv, authSvc, _ := setupAuthServer(t)
	tok, _, err := authSvc.GenerateTokenPair("user-1", "npub1querydep")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" + tok
	conn, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	if resp.StatusCode != 101 {
		t.Fatalf("status=%d want 101", resp.StatusCode)
	}
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(buf.String(), "ws query token deprecated path=/ws") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	logged := buf.String()
	if !strings.Contains(logged, "ws query token deprecated path=/ws") {
		t.Fatalf("missing deprecation warning: %s", logged)
	}
	if strings.Contains(logged, tok) {
		t.Fatal("query token value leaked into log")
	}
}

func TestWS_NilAuthIgnoresTokenAsUser(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("DATA_DIR", dbDir)
	s := &Server{}
	s.initHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	secret := "this-must-not-become-userid"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?userId=alice"
	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"proxi", "proxi-jwt." + secret},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	if resp.StatusCode != 101 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if resp.Header.Get("Sec-WebSocket-Protocol") != "proxi" {
		t.Fatalf("echo=%q", resp.Header.Get("Sec-WebSocket-Protocol"))
	}
	deadline := time.Now().Add(time.Second)
	var users []string
	for time.Now().Before(deadline) {
		users = s.hub.OnlineUsers()
		if len(users) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	joined := strings.Join(users, ",")
	if !strings.Contains(joined, "alice") {
		t.Fatalf("online=%v", users)
	}
	if strings.Contains(joined, secret) {
		t.Fatal("token became userId")
	}
}
