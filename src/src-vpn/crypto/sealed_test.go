package crypto

import (
	"bytes"
	"testing"
)

func TestSealUnsealRoundtrip(t *testing.T) {
	senderPriv, senderPub, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("generate sender key: %v", err)
	}
	recipientPriv, recipientPub, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("generate recipient key: %v", err)
	}

	plaintext := []byte("Hello, sealed sender!")

	sealed, err := SealMessage(plaintext, senderPriv[:], recipientPub[:])
	if err != nil {
		t.Fatalf("SealMessage: %v", err)
	}

	decrypted, recoveredSenderPub, err := UnsealMessage(sealed, recipientPriv[:])
	if err != nil {
		t.Fatalf("UnsealMessage: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
	if !bytes.Equal(recoveredSenderPub, senderPub[:]) {
		t.Error("recovered sender pub does not match original")
	}
}

func TestSealUnsealWrongKey(t *testing.T) {
	senderPriv, _, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("generate sender key: %v", err)
	}
	_, recipientPub, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("generate recipient key: %v", err)
	}

	plaintext := []byte("secret message")
	sealed, err := SealMessage(plaintext, senderPriv[:], recipientPub[:])
	if err != nil {
		t.Fatalf("SealMessage: %v", err)
	}

	// Generate a different (wrong) private key
	wrongPriv, _, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("generate wrong key: %v", err)
	}

	_, _, err = UnsealMessage(sealed, wrongPriv[:])
	if err == nil {
		t.Error("expected error when unsealing with wrong key")
	}
}

func TestSealUnsealIdentifySender(t *testing.T) {
	// Generate two senders and one recipient
	sender1Priv, sender1Pub, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("generate sender1 key: %v", err)
	}
	sender2Priv, sender2Pub, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("generate sender2 key: %v", err)
	}
	recipientPriv, recipientPub, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("generate recipient key: %v", err)
	}

	msg := []byte("identifiable message")

	// Sender 1 seals a message to recipient
	sealed1, err := SealMessage(msg, sender1Priv[:], recipientPub[:])
	if err != nil {
		t.Fatalf("SealMessage sender1: %v", err)
	}

	// Sender 2 seals a message to recipient
	sealed2, err := SealMessage(msg, sender2Priv[:], recipientPub[:])
	if err != nil {
		t.Fatalf("SealMessage sender2: %v", err)
	}

	// Recipient unseals message 1 and identifies sender 1
	_, sender1Recovered, err := UnsealMessage(sealed1, recipientPriv[:])
	if err != nil {
		t.Fatalf("UnsealMessage sender1: %v", err)
	}
	if !bytes.Equal(sender1Recovered, sender1Pub[:]) {
		t.Error("sender1 identity mismatch after unseal")
	}

	// Recipient unseals message 2 and identifies sender 2
	_, sender2Recovered, err := UnsealMessage(sealed2, recipientPriv[:])
	if err != nil {
		t.Fatalf("UnsealMessage sender2: %v", err)
	}
	if !bytes.Equal(sender2Recovered, sender2Pub[:]) {
		t.Error("sender2 identity mismatch after unseal")
	}

	// Different senders must produce different recovered keys
	if bytes.Equal(sender1Recovered, sender2Recovered) {
		t.Error("two different senders should produce different recovered public keys")
	}
}
