package chat

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/unkillable-messenger/vpn/crypto"
	"github.com/unkillable-messenger/vpn/store"
)

// helperGenerateKeyPair creates a DH key pair and returns a PreKeyBundle
// where IdentityKey holds the X25519 private key and SignedPreKey holds the
// X25519 public key, plus the hex-encoded public key string.
func helperGenerateKeyPair(t *testing.T) (*crypto.PreKeyBundle, string) {
	t.Helper()
	priv, pub, err := crypto.GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("GenerateDHKeyPair: %v", err)
	}
	bundle := &crypto.PreKeyBundle{
		IdentityKey:  priv[:],
		SignedPreKey: pub[:],
	}
	return bundle, hex.EncodeToString(pub[:])
}

// TestEncryptDecryptRoundtrip verifies that a message encrypted by the sender
// can be decrypted by the intended recipient and produces the original plaintext.
func TestEncryptDecryptRoundtrip(t *testing.T) {
	senderBundle, _ := helperGenerateKeyPair(t)
	recipientBundle, recipientPubHex := helperGenerateKeyPair(t)

	_, senderPubHex := helperGenerateKeyPair(t)
	// We already have senderBundle, so re-use its public key.
	senderPubHex = hex.EncodeToString(senderBundle.SignedPreKey)

	message := "Hello, E2E encrypted world!"

	encrypted, err := EncryptMessageForRecipient(message, senderBundle, recipientPubHex)
	if err != nil {
		t.Fatalf("EncryptMessageForRecipient: %v", err)
	}

	if encrypted == "" {
		t.Fatal("encrypted message should not be empty")
	}

	if encrypted == message {
		t.Fatal("encrypted message should differ from plaintext")
	}

	plaintext, err := DecryptMessageFromSender(encrypted, recipientBundle, senderPubHex)
	if err != nil {
		t.Fatalf("DecryptMessageFromSender: %v", err)
	}

	if plaintext != message {
		t.Fatalf("decrypted message mismatch: got %q, want %q", plaintext, message)
	}
}

// TestEncryptDecryptWrongKey verifies that attempting to decrypt a message
// with the wrong (third-party) key fails.
func TestEncryptDecryptWrongKey(t *testing.T) {
	senderBundle, _ := helperGenerateKeyPair(t)
	_, recipientPubHex := helperGenerateKeyPair(t)
	eavesdropperBundle, _ := helperGenerateKeyPair(t)

	senderPubHex := hex.EncodeToString(senderBundle.SignedPreKey)

	message := "Secret message for the real recipient"

	encrypted, err := EncryptMessageForRecipient(message, senderBundle, recipientPubHex)
	if err != nil {
		t.Fatalf("EncryptMessageForRecipient: %v", err)
	}

	// The eavesdropper tries to decrypt with their own key — must fail.
	_, err = DecryptMessageFromSender(encrypted, eavesdropperBundle, senderPubHex)
	if err == nil {
		t.Fatal("decrypting with wrong key should have failed, but succeeded")
	}
}

// TestE2ESessionCreation verifies that an E2ESession can be created with valid
// npubs and that it contains a non-nil, 32-byte shared secret.
func TestE2ESessionCreation(t *testing.T) {
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer db.Close()

	session, err := GenerateE2ESession("npub1alice", "npub1bob", db)
	if err != nil {
		t.Fatalf("GenerateE2ESession: %v", err)
	}

	if session.LocalNpub != "npub1alice" {
		t.Errorf("LocalNpub = %q, want %q", session.LocalNpub, "npub1alice")
	}
	if session.RemoteNpub != "npub1bob" {
		t.Errorf("RemoteNpub = %q, want %q", session.RemoteNpub, "npub1bob")
	}
	if len(session.SharedSecret) != 32 {
		t.Errorf("SharedSecret length = %d, want 32", len(session.SharedSecret))
	}
	if session.CreatedAt == 0 {
		t.Error("CreatedAt should not be zero")
	}
}

// TestEncryptEmptyMessage verifies that an empty string can be encrypted
// and decrypted without error.
func TestEncryptEmptyMessage(t *testing.T) {
	senderBundle, _ := helperGenerateKeyPair(t)
	recipientBundle, recipientPubHex := helperGenerateKeyPair(t)
	senderPubHex := hex.EncodeToString(senderBundle.SignedPreKey)

	encrypted, err := EncryptMessageForRecipient("", senderBundle, recipientPubHex)
	if err != nil {
		t.Fatalf("EncryptMessageForRecipient (empty): %v", err)
	}

	plaintext, err := DecryptMessageFromSender(encrypted, recipientBundle, senderPubHex)
	if err != nil {
		t.Fatalf("DecryptMessageFromSender (empty): %v", err)
	}

	if plaintext != "" {
		t.Fatalf("decrypted empty message should be empty, got %q", plaintext)
	}
}

// TestEncryptLargeMessage verifies that a large (100 KB) message can be
// encrypted and decrypted correctly.
func TestEncryptLargeMessage(t *testing.T) {
	senderBundle, _ := helperGenerateKeyPair(t)
	recipientBundle, recipientPubHex := helperGenerateKeyPair(t)
	senderPubHex := hex.EncodeToString(senderBundle.SignedPreKey)

	// Build a ~100 KB message.
	largeMessage := strings.Repeat("A", 100*1024) // 102400 bytes

	encrypted, err := EncryptMessageForRecipient(largeMessage, senderBundle, recipientPubHex)
	if err != nil {
		t.Fatalf("EncryptMessageForRecipient (100KB): %v", err)
	}

	plaintext, err := DecryptMessageFromSender(encrypted, recipientBundle, senderPubHex)
	if err != nil {
		t.Fatalf("DecryptMessageFromSender (100KB): %v", err)
	}

	if len(plaintext) != len(largeMessage) {
		t.Fatalf("decrypted length = %d, want %d", len(plaintext), len(largeMessage))
	}

	if plaintext != largeMessage {
		// Avoid printing 100 KB on failure; just show length mismatch.
		t.Fatalf("decrypted 100KB message content mismatch (first 64 chars: %q...)", truncate(plaintext, 64))
	}
}

// truncate returns the first n characters of s for safe test output.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return fmt.Sprintf("%s...", s[:n])
}
