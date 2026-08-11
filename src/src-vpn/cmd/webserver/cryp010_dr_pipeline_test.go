package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/crypto"
	"github.com/unkillable-messenger/vpn/store"
)

// CRYP-010: DR encrypt before SaveMessage; SQLite holds ciphertext; Bob decrypts to original.
func TestCRYP010_MessagePipelineStoresCiphertext(t *testing.T) {
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	dr := chat.NewDRSessionStore()
	bobMat, err := crypto.NewX3DHBobMaterial()
	if err != nil {
		t.Fatal(err)
	}
	aliceIK, err := crypto.GenerateX3DHKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := dr.BootstrapPair("npub_alice", "npub_bob", aliceIK, bobMat); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	srv := &Server{db: db, hub: chat.NewChatHub(), drSessions: dr}

	plain := "forward-secret CRYP-010"
	body := `{"from":"npub_alice","to":"npub_bob","text":"` + plain + `"}`
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
	if created.Text == plain {
		t.Fatal("API response must not return plaintext when DR used — expected ciphertext")
	}
	if !created.Encrypted {
		t.Fatal("Encrypted flag must be true")
	}
	if !chat.IsDRCiphertext(created.Text) {
		t.Fatalf("stored text must be dr1: ciphertext, got prefix %q", created.Text[:minInt(40, len(created.Text))])
	}

	// Direct DB read — must be ciphertext
	msgs, err := db.GetMessages(10, 0, "npub_bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) == 0 {
		t.Fatal("no messages in DB")
	}
	dbMsg := msgs[0]
	if dbMsg.Text == plain {
		t.Fatal("SQLite must store ciphertext, found plaintext")
	}
	if !chat.IsDRCiphertext(dbMsg.Text) {
		t.Fatalf("DB text not DR ciphertext: %q", dbMsg.Text[:minInt(40, len(dbMsg.Text))])
	}

	// Do NOT decrypt here — would advance ratchet and break GET auto-decrypt.
	// Ciphertext-in-DB already proven above.

	// GET API as Bob should auto-decrypt once
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
	if resp.Messages[0].Text != plain {
		t.Fatalf("GET decrypt got %q want %q", resp.Messages[0].Text, plain)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
