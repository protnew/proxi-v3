package chat

import (
	"testing"

	"github.com/unkillable-messenger/vpn/crypto"
)

func TestCRYP010_DREncryptBeforePersistRoundtrip(t *testing.T) {
	store := NewDRSessionStore()
	bobMat, err := crypto.NewX3DHBobMaterial()
	if err != nil {
		t.Fatal(err)
	}
	aliceIK, err := crypto.GenerateX3DHKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BootstrapPair("alice", "bob", aliceIK, bobMat); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	plain := "secret message CRYP-010"
	ct, used, err := store.EncryptOutbound("alice", "bob", plain)
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("expected DR to be used")
	}
	if ct == plain {
		t.Fatal("ciphertext must differ from plaintext")
	}
	if !IsDRCiphertext(ct) {
		t.Fatalf("expected dr1: prefix, got %q", ct[:20])
	}

	// Simulate SQLite store of ciphertext only
	stored := ct

	got, usedDec, err := store.DecryptInbound("bob", "alice", stored)
	if err != nil {
		t.Fatal(err)
	}
	if !usedDec {
		t.Fatal("expected DR decrypt")
	}
	if got != plain {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestCRYP010_NoSessionPassthrough(t *testing.T) {
	store := NewDRSessionStore()
	ct, used, err := store.EncryptOutbound("a", "b", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if used {
		t.Fatal("no session → DR must not be used")
	}
	if ct != "hello" {
		t.Fatalf("passthrough failed: %q", ct)
	}
}
