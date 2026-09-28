package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/crypto"
	"github.com/unkillable-messenger/vpn/store"
)

func TestCRYP011_X3DHCalledOnSessionCreate(t *testing.T) {
	// Unit-level: CreateSessionAsInitiator must invoke X3DH
	dr := chat.NewDRSessionStore()
	bobMat, err := crypto.NewX3DHBobMaterial()
	if err != nil {
		t.Fatal(err)
	}
	aliceIK, err := crypto.GenerateX3DHKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	ek, err := dr.CreateSessionAsInitiator("alice", "bob", aliceIK, bobMat)
	if err != nil {
		t.Fatalf("CreateSessionAsInitiator (X3DH): %v", err)
	}
	var zero [32]byte
	if ek == zero {
		t.Fatal("EK pub must be non-zero after X3DH")
	}
	if !dr.HasSession("alice", "bob") {
		t.Fatal("session not stored after X3DH")
	}
	if err := dr.CreateSessionAsResponder("bob", "alice", bobMat, aliceIK.Pub, ek); err != nil {
		t.Fatalf("CreateSessionAsResponder: %v", err)
	}
	if !dr.HasSession("bob", "alice") {
		t.Fatal("bob session missing after X3DH respond")
	}
}

func TestCRYP011_HTTPSessionEstablishThenDRMessage(t *testing.T) {
	t.Setenv("VPN_DEV_BOOTSTRAP_PAIR", "1") // P15: bootstrap_pair gated by dev flag
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	srv := &Server{db: db, hub: chat.NewChatHub(), drSessions: chat.NewDRSessionStore()}

	body := `{"local_npub":"npub_a","remote_npub":"npub_b","mode":"bootstrap_pair"}`
	req := httptest.NewRequest(http.MethodPost, "/api/keys/session", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleSessionEstablish(w, req)
	if w.Code != 201 {
		t.Fatalf("establish status %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["x3dh"] != true || resp["double_ratchet"] != true {
		t.Fatalf("response flags: %v", resp)
	}
	if !srv.drSessions.HasSession("npub_a", "npub_b") {
		t.Fatal("no session after establish")
	}

	// X2: message path is client-blind — POST client ciphertext (not server DR encrypt).
	cipher := "nip44:c2Vzcy1vay1jaXBoZXI="
	msgBody := `{"from":"npub_a","to":"npub_b","text":"` + cipher + `","encrypted":true}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(msgBody))
	w2 := httptest.NewRecorder()
	srv.handleMessagesPost(w2, req2)
	if w2.Code != 201 {
		t.Fatalf("msg status %d body=%s", w2.Code, w2.Body.String())
	}
	var msg store.Message
	_ = json.Unmarshal(w2.Body.Bytes(), &msg)
	if !chat.LooksLikeClientCiphertext(msg.Text) {
		t.Fatalf("expected client ciphertext after X2, got %q", msg.Text)
	}
	if msg.Text != cipher {
		t.Fatalf("want stored %q got %q", cipher, msg.Text)
	}
}
