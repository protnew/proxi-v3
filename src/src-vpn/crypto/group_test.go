package crypto

import (
	"crypto/rand"
	"testing"
)

func TestGroupSession_CreateAndEncrypt(t *testing.T) {
	t.Parallel()

	groupID := "test-group-1"
	members := []string{"alice", "bob", "charlie"}

	// Generate keys for members
	privAlice, pubAlice, _ := GenerateDHKeyPair()
	_, pubBob, _ := GenerateDHKeyPair()
	_, pubCharlie, _ := GenerateDHKeyPair()

	peerPubs := map[string][32]byte{
		"bob":     pubBob,
		"charlie": pubCharlie,
		"alice":   pubAlice, // include self for completeness
	}

	gs, err := CreateGroupSession(groupID, members, privAlice, peerPubs)
	if err != nil {
		t.Fatalf("CreateGroupSession: %v", err)
	}

	// Verify all members have sender keys
	for _, m := range members {
		if !gs.HasMember(m) {
			t.Fatalf("member %s should have a sender key", m)
		}
	}

	// Alice encrypts a message
	plaintext := []byte("Hello group! This is Alice.")
	ciphertext, err := gs.Encrypt("alice", plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if len(ciphertext) == 0 {
		t.Fatal("ciphertext should not be empty")
	}

	// Bob decrypts using Alice's sender key
	decrypted, err := gs.Decrypt("alice", ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestGroupSession_MultipleSenders(t *testing.T) {
	t.Parallel()

	groupID := "test-group-2"
	members := []string{"alice", "bob"}
	privAlice, _, _ := GenerateDHKeyPair()
	_, pubBob, _ := GenerateDHKeyPair()

	peerPubs := map[string][32]byte{"bob": pubBob}

	gs, err := CreateGroupSession(groupID, members, privAlice, peerPubs)
	if err != nil {
		t.Fatalf("CreateGroupSession: %v", err)
	}

	msgAlice := []byte("Hi from Alice!")
	msgBob := []byte("Hi from Bob!")

	ctAlice, err := gs.Encrypt("alice", msgAlice)
	if err != nil {
		t.Fatalf("Encrypt alice: %v", err)
	}

	ctBob, err := gs.Encrypt("bob", msgBob)
	if err != nil {
		t.Fatalf("Encrypt bob: %v", err)
	}

	// Decrypt Alice's message
	decAlice, err := gs.Decrypt("alice", ctAlice)
	if err != nil {
		t.Fatalf("Decrypt alice: %v", err)
	}
	if string(decAlice) != string(msgAlice) {
		t.Fatalf("alice msg mismatch: expected %q, got %q", msgAlice, decAlice)
	}

	// Decrypt Bob's message
	decBob, err := gs.Decrypt("bob", ctBob)
	if err != nil {
		t.Fatalf("Decrypt bob: %v", err)
	}
	if string(decBob) != string(msgBob) {
		t.Fatalf("bob msg mismatch: expected %q, got %q", msgBob, decBob)
	}
}

func TestGroupSession_DecryptAny(t *testing.T) {
	t.Parallel()

	members := []string{"alice", "bob", "charlie"}
	privAlice, _, _ := GenerateDHKeyPair()

	gs, err := CreateGroupSession("g3", members, privAlice, nil)
	if err != nil {
		t.Fatalf("CreateGroupSession: %v", err)
	}

	plaintext := []byte("Who sent this?")
	ct, err := gs.Encrypt("bob", plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// DecryptAny should identify bob as the sender
	senderID, decrypted, err := gs.DecryptAny(ct)
	if err != nil {
		t.Fatalf("DecryptAny: %v", err)
	}
	if senderID != "bob" {
		t.Fatalf("expected sender bob, got %s", senderID)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("plaintext mismatch")
	}
}

func TestGroupSession_EncryptUnknownSender(t *testing.T) {
	t.Parallel()

	gs := NewGroupSession("g4")
	_, err := gs.Encrypt("unknown_user", []byte("test"))
	if err == nil {
		t.Fatal("expected error for unknown sender")
	}
}

func TestGroupSession_DecryptUnknownSender(t *testing.T) {
	t.Parallel()

	gs := NewGroupSession("g5")
	_, err := gs.Decrypt("unknown_user", make([]byte, 100))
	if err == nil {
		t.Fatal("expected error for unknown sender")
	}
}

func TestGroupSession_AddRemoveMember(t *testing.T) {
	t.Parallel()

	gs := NewGroupSession("g6")

	// Add alice
	var senderKey [32]byte
	rand.Read(senderKey[:])
	gs.AddMember("alice", nil, nil)

	if !gs.HasMember("alice") {
		t.Fatal("alice should be a member")
	}

	members := gs.GetMembers()
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}

	// Add bob
	gs.AddMember("bob", nil, nil)
	if len(gs.GetMembers()) != 2 {
		t.Fatalf("expected 2 members, got %d", len(gs.GetMembers()))
	}

	// Remove alice
	gs.RemoveMember("alice")
	if gs.HasMember("alice") {
		t.Fatal("alice should be removed")
	}
	if len(gs.GetMembers()) != 1 {
		t.Fatalf("expected 1 member after removal, got %d", len(gs.GetMembers()))
	}
}

func TestGroupSession_RotateSenderKey(t *testing.T) {
	t.Parallel()

	gs, err := CreateGroupSession("g7", []string{"alice", "bob"}, [32]byte{}, nil)
	if err != nil {
		t.Fatalf("CreateGroupSession: %v", err)
	}

	// Encrypt with original key
	ct1, err := gs.Encrypt("alice", []byte("before rotation"))
	if err != nil {
		t.Fatalf("Encrypt before rotation: %v", err)
	}

	// Rotate alice's key
	if err := gs.RotateSenderKey("alice"); err != nil {
		t.Fatalf("RotateSenderKey: %v", err)
	}

	// Decrypt old ciphertext should still work if we kept old key (but we replaced it)
	// Actually with rotation, old ciphertext can't be decrypted with new key
	_, err = gs.Decrypt("alice", ct1)
	if err == nil {
		t.Fatal("expected decrypt to fail with rotated key")
	}

	// Encrypt with new key should work
	ct2, err := gs.Encrypt("alice", []byte("after rotation"))
	if err != nil {
		t.Fatalf("Encrypt after rotation: %v", err)
	}

	dec, err := gs.Decrypt("alice", ct2)
	if err != nil {
		t.Fatalf("Decrypt after rotation: %v", err)
	}
	if string(dec) != "after rotation" {
		t.Fatalf("unexpected plaintext: %q", dec)
	}
}

func TestGroupSession_DecryptTamperedCiphertext(t *testing.T) {
	t.Parallel()

	gs, err := CreateGroupSession("g8", []string{"alice"}, [32]byte{}, nil)
	if err != nil {
		t.Fatalf("CreateGroupSession: %v", err)
	}

	ct, err := gs.Encrypt("alice", []byte("secret message"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Tamper with ciphertext
	if len(ct) > 20 {
		ct[20] ^= 0xFF
	}

	_, err = gs.Decrypt("alice", ct)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestGroupSession_DecryptAnyNoMatch(t *testing.T) {
	t.Parallel()

	gs := NewGroupSession("g9")

	// Random garbage ciphertext
	ct := make([]byte, 100)
	rand.Read(ct)

	_, _, err := gs.DecryptAny(ct)
	if err == nil {
		t.Fatal("expected error with no members")
	}
}

func TestNewGroupSession(t *testing.T) {
	t.Parallel()

	gs := NewGroupSession("test-group")
	if gs == nil {
		t.Fatal("NewGroupSession returned nil")
	}
	if gs.GroupID != "test-group" {
		t.Fatalf("expected GroupID 'test-group', got %q", gs.GroupID)
	}
	if gs.senderKeys == nil || gs.chainKeys == nil {
		t.Fatal("internal maps should be initialized")
	}
}

func TestGroupSession_EmptyPlaintext(t *testing.T) {
	t.Parallel()

	gs, err := CreateGroupSession("g10", []string{"alice"}, [32]byte{}, nil)
	if err != nil {
		t.Fatalf("CreateGroupSession: %v", err)
	}

	ct, err := gs.Encrypt("alice", []byte{})
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}

	dec, err := gs.Decrypt("alice", ct)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if len(dec) != 0 {
		t.Fatalf("expected empty plaintext, got %d bytes", len(dec))
	}
}

func TestGroupSession_LargePlaintext(t *testing.T) {
	t.Parallel()

	gs, err := CreateGroupSession("g11", []string{"alice"}, [32]byte{}, nil)
	if err != nil {
		t.Fatalf("CreateGroupSession: %v", err)
	}

	// 100KB message
	largeMsg := make([]byte, 100*1024)
	rand.Read(largeMsg)

	ct, err := gs.Encrypt("alice", largeMsg)
	if err != nil {
		t.Fatalf("Encrypt large: %v", err)
	}

	dec, err := gs.Decrypt("alice", ct)
	if err != nil {
		t.Fatalf("Decrypt large: %v", err)
	}
	if len(dec) != len(largeMsg) {
		t.Fatalf("size mismatch: expected %d, got %d", len(largeMsg), len(dec))
	}
}
