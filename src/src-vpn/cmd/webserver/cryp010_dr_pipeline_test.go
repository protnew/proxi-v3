package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/store"
)

// CRYP-010 (X2 2026-09-23): server is blind — stores client ciphertext as-is;
// GET returns ciphertext (no server auto-decrypt).
func TestCRYP010_MessagePipelineStoresCiphertext(t *testing.T) {
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := &Server{db: db, hub: chat.NewChatHub()}

	cipher := "nip44:Zm9yd2FyZC1zZWNyZXQgQ1JZUC0wMTA="
	body := `{"from":"npub_alice","to":"npub_bob","text":"` + cipher + `","encrypted":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleMessagesPost(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST status %d body=%s", w.Code, w.Body.String())
	}

	var created store.Message
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Text != cipher {
		t.Fatalf("API must echo client ciphertext, got %q", created.Text)
	}
	if !created.Encrypted {
		t.Fatal("Encrypted flag must be true")
	}
	if !chat.LooksLikeClientCiphertext(created.Text) {
		t.Fatalf("stored text must look like client ciphertext, got %q", created.Text)
	}

	msgs, err := db.GetMessages(10, 0, "npub_bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) == 0 {
		t.Fatal("no messages in DB")
	}
	dbMsg := msgs[0]
	if dbMsg.Text != cipher {
		t.Fatalf("SQLite must store client ciphertext, got %q", dbMsg.Text)
	}

	// GET as Bob — server blind: ciphertext returned unchanged
	reqGet := httptest.NewRequest(http.MethodGet, "/api/messages?npub=npub_bob", nil)
	ctx := context.WithValue(reqGet.Context(), "npub", "npub_bob")
	reqGet = reqGet.WithContext(ctx)
	wGet := httptest.NewRecorder()
	srv.handleMessagesGet(wGet, reqGet)
	if wGet.Code != 200 {
		t.Fatalf("GET status %d body=%s", wGet.Code, wGet.Body.String())
	}
	var resp struct {
		Messages []store.Message `json:"messages"`
	}
	if err := json.Unmarshal(wGet.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Messages) == 0 {
		t.Fatal("GET empty")
	}
	if resp.Messages[0].Text != cipher {
		t.Fatalf("GET must return ciphertext (server blind), got %q want %q", resp.Messages[0].Text, cipher)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
