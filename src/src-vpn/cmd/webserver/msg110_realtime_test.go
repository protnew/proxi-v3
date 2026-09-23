package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/store"
)

// MSG-110: REST POST /api/messages must push via hub.SendTo so online peers
// receive without page reload. X2: payload must be client ciphertext.
func TestMSG110_RESTPostPushesToWebSocketHub(t *testing.T) {
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	hub := chat.NewChatHub()
	srv := &Server{db: db, hub: hub}

	bobRecv := make(chan []byte, 16)
	bob := &chat.Client{
		UserID: "npub_bob_test",
		Send:   bobRecv,
	}
	hub.Register(bob)

	cipher := "nip44:aGVsbG8gcmVhbC10aW1lIE1TRy0xMTA="
	body := `{"from":"npub_alice_test","to":"npub_bob_test","text":"` + cipher + `","encrypted":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMessagesPost(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	var got chat.Message
	found := false
	for time.Now().Before(deadline) && !found {
		select {
		case data := <-bobRecv:
			var msg chat.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.Type == chat.TypeChat {
				got = msg
				found = true
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !found {
		t.Fatal("MSG-110 FAIL: Bob received no chat frame on hub within 2s after REST POST")
	}
	if got.To != "npub_bob_test" {
		t.Errorf("to=%q want npub_bob_test", got.To)
	}
	if got.From != "npub_alice_test" {
		t.Errorf("from=%q want npub_alice_test", got.From)
	}
	if got.Text != cipher {
		t.Fatalf("hub text=%q want client ciphertext %q", got.Text, cipher)
	}
	if got.ID == "" {
		t.Error("message id empty")
	}
}

func TestMSG110_OfflineRecipientStillPersists(t *testing.T) {
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	hub := chat.NewChatHub()
	srv := &Server{db: db, hub: hub}

	cipher := "nip44:b2ZmbGluZSBvayBjaXBoZXI="
	body := `{"from":"npub_alice_test","to":"npub_charlie_offline","text":"` + cipher + `","encrypted":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleMessagesPost(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("offline recipient: expected 201, got %d body=%s", w.Code, w.Body.String())
	}
}

// X2: plaintext DM must be rejected (server blind).
func TestMSG110_PlaintextDMForbidden(t *testing.T) {
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	srv := &Server{db: db, hub: chat.NewChatHub()}
	body := `{"from":"npub_alice_test","to":"npub_bob_test","text":"plain forbidden"}`
	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleMessagesPost(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "PLAINTEXT_DM_FORBIDDEN") {
		t.Fatalf("body missing PLAINTEXT_DM_FORBIDDEN: %s", w.Body.String())
	}
}
