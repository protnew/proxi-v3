package crypto

import (
	"testing"
)

func TestGenerateDHKeyPair(t *testing.T) {
	priv, pub, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("GenerateDHKeyPair: %v", err)
	}
	if len(priv) != 32 {
		t.Errorf("private key length = %d, want 32", len(priv))
	}
	if len(pub) != 32 {
		t.Errorf("public key length = %d, want 32", len(pub))
	}

	// Two key pairs should be different
	priv2, pub2, _ := GenerateDHKeyPair()
	if priv == priv2 {
		t.Error("two private keys should differ")
	}
	if pub == pub2 {
		t.Error("two public keys should differ")
	}
}

func TestComputeSharedSecret(t *testing.T) {
	// Generate two key pairs: A and B
	privA, pubA, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	privB, pubB, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// A computes shared with B's public key
	sharedAB, err := ComputeSharedSecret(privA, pubB)
	if err != nil {
		t.Fatalf("ComputeSharedSecret A→B: %v", err)
	}

	// B computes shared with A's public key
	sharedBA, err := ComputeSharedSecret(privB, pubA)
	if err != nil {
		t.Fatalf("ComputeSharedSecret B→A: %v", err)
	}

	// Commutative property: both should be equal
	if sharedAB != sharedBA {
		t.Error("shared secrets should be equal (commutative)")
	}
}

func TestDeriveChatKey(t *testing.T) {
	privA, pubA, _ := GenerateDHKeyPair()
	privB, pubB, _ := GenerateDHKeyPair()

	shared, _ := ComputeSharedSecret(privA, pubB)

	// Same context = same key
	key1 := DeriveChatKey(shared, "chat-123")
	key2 := DeriveChatKey(shared, "chat-123")
	if key1 != key2 {
		t.Error("same context should produce same chat key")
	}

	// Different context = different key
	key3 := DeriveChatKey(shared, "chat-456")
	if key1 == key3 {
		t.Error("different context should produce different chat key")
	}

	// Different shared secret = different key
	shared2, _ := ComputeSharedSecret(privB, pubA)
	key4 := DeriveChatKey(shared2, "chat-123")
	if key1 != key4 {
		t.Error("same shared secret + same context should produce same key (commutativity)")
	}
}
