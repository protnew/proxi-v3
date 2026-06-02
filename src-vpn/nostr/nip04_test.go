package nostr

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
)

// ==================== NIP-04 Encrypt/Decrypt Tests ====================

func TestEncryptDecrypt(t *testing.T) {
	// Generate sender and recipient key pairs
	senderPriv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("generate sender key: %v", err)
	}
	recipientPriv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("generate recipient key: %v", err)
	}
	senderPub := senderPriv.PubKey()
	recipientPub := recipientPriv.PubKey()

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple message", "Hello, World!"},
		{"empty string", ""},
		{"unicode", "Привет мир 🌍 日本語テスト"},
		{"exactly 16 bytes", "1234567890123456"},
		{"15 bytes", "123456789012345"},
		{"17 bytes", "12345678901234567"},
		{"long message", "This is a much longer message that spans multiple AES blocks and tests that the CBC encryption handles larger payloads correctly. Lorem ipsum dolor sit amet, consectetur adipiscing elit."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt: sender encrypts with their private key + recipient's public key
			ciphertext, err := Encrypt(senderPriv, recipientPub, tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			if ciphertext == "" && tt.plaintext != "" {
				t.Error("expected non-empty ciphertext")
			}

			// Decrypt: recipient decrypts with their private key + sender's public key
			plaintext, err := Decrypt(recipientPriv, senderPub, ciphertext)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if plaintext != tt.plaintext {
				t.Errorf("plaintext mismatch:\nexpected: %q\n got: %q", tt.plaintext, plaintext)
			}
		})
	}
}

func TestEncryptDecryptReverse(t *testing.T) {
	// Test that the recipient can also encrypt and the sender can decrypt
	senderPriv, _ := btcec.NewPrivateKey()
	recipientPriv, _ := btcec.NewPrivateKey()
	senderPub := senderPriv.PubKey()
	recipientPub := recipientPriv.PubKey()

	message := "Reply from recipient"

	// Recipient encrypts with their private key + sender's public key
	ciphertext, err := Encrypt(recipientPriv, senderPub, message)
	if err != nil {
		t.Fatalf("Encrypt (reverse) failed: %v", err)
	}

	// Sender decrypts with their private key + recipient's public key
	plaintext, err := Decrypt(senderPriv, recipientPub, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt (reverse) failed: %v", err)
	}

	if plaintext != message {
		t.Errorf("reverse decrypt mismatch: expected %q, got %q", message, plaintext)
	}
}

func TestEncryptDecryptDifferentKeys_fails(t *testing.T) {
	// Generate three key pairs: sender, recipient, and an eavesdropper
	senderPriv, _ := btcec.NewPrivateKey()
	recipientPriv, _ := btcec.NewPrivateKey()
	eavesdropperPriv, _ := btcec.NewPrivateKey()

	recipientPub := recipientPriv.PubKey()
	eavesdropperPub := eavesdropperPriv.PubKey()

	message := "Secret message"

	// Sender encrypts for the recipient
	ciphertext, err := Encrypt(senderPriv, recipientPub, message)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Eavesdropper tries to decrypt with their own private key — should fail
	_, err = Decrypt(eavesdropperPriv, senderPriv.PubKey(), ciphertext)
	if err == nil {
		t.Error("expected decryption to fail with wrong private key, but it succeeded")
	}

	// Eavesdropper tries to decrypt with the right private key but wrong sender pubkey — should fail
	_, err = Decrypt(recipientPriv, eavesdropperPub, ciphertext)
	if err == nil {
		t.Error("expected decryption to fail with wrong sender public key, but it succeeded")
	}

	// Correct decryption should still work
	plaintext, err := Decrypt(recipientPriv, senderPriv.PubKey(), ciphertext)
	if err != nil {
		t.Fatalf("correct Decrypt failed: %v", err)
	}
	if plaintext != message {
		t.Errorf("plaintext mismatch: expected %q, got %q", message, plaintext)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	// Encrypting the same message twice should produce different ciphertexts
	// because the IV is random
	senderPriv, _ := btcec.NewPrivateKey()
	recipientPriv, _ := btcec.NewPrivateKey()
	recipientPub := recipientPriv.PubKey()

	message := "Same message"

	ct1, err := Encrypt(senderPriv, recipientPub, message)
	if err != nil {
		t.Fatalf("Encrypt 1 failed: %v", err)
	}
	ct2, err := Encrypt(senderPriv, recipientPub, message)
	if err != nil {
		t.Fatalf("Encrypt 2 failed: %v", err)
	}

	if ct1 == ct2 {
		t.Error("expected different ciphertexts due to random IV, but got identical")
	}

	// Both should decrypt to the same plaintext
	pt1, err := Decrypt(recipientPriv, senderPriv.PubKey(), ct1)
	if err != nil {
		t.Fatalf("Decrypt 1 failed: %v", err)
	}
	pt2, err := Decrypt(recipientPriv, senderPriv.PubKey(), ct2)
	if err != nil {
		t.Fatalf("Decrypt 2 failed: %v", err)
	}

	if pt1 != message || pt2 != message {
		t.Errorf("decrypted messages don't match original: %q, %q", pt1, pt2)
	}
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	priv1, _ := btcec.NewPrivateKey()
	priv2, _ := btcec.NewPrivateKey()
	pub2 := priv2.PubKey()

	tests := []struct {
		name       string
		ciphertext string
	}{
		{"empty string", ""},
		{"not base64", "!!!not-base64!!!"},
		{"too short", "AA=="},
		{"valid base64 but too short for IV", "AQID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(priv1, pub2, tt.ciphertext)
			if err == nil {
				t.Error("expected error for invalid ciphertext, got nil")
			}
		})
	}
}

// ==================== NIP-04 Event Format Tests ====================

func TestSendDirectMessage_format(t *testing.T) {
	senderPriv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("generate sender key: %v", err)
	}
	recipientPriv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("generate recipient key: %v", err)
	}

	senderPrivHex := hex.EncodeToString(senderPriv.Serialize())
	recipientPubHex := hex.EncodeToString(recipientPriv.PubKey().SerializeCompressed())

	message := "Hello from NIP-04!"

	// Create event with nil relay (just builds the event without sending)
	evt, err := SendDirectMessage(nil, senderPrivHex, recipientPubHex, message)
	if err != nil {
		t.Fatalf("SendDirectMessage failed: %v", err)
	}

	// Verify event fields
	if evt.Kind != 4 {
		t.Errorf("expected kind=4, got kind=%d", evt.Kind)
	}

	if evt.PubKey != hex.EncodeToString(senderPriv.PubKey().SerializeCompressed()) {
		t.Errorf("expected pubkey=%s, got %s",
			hex.EncodeToString(senderPriv.PubKey().SerializeCompressed()), evt.PubKey)
	}

	if evt.CreatedAt == 0 {
		t.Error("expected non-zero created_at")
	}

	// Verify p-tag (recipient)
	if len(evt.Tags) < 1 {
		t.Fatal("expected at least 1 tag")
	}
	if evt.Tags[0][0] != "p" {
		t.Errorf("expected first tag type 'p', got %q", evt.Tags[0][0])
	}
	if evt.Tags[0][1] != recipientPubHex {
		t.Errorf("expected p-tag to be recipient pubkey %s, got %s", recipientPubHex, evt.Tags[0][1])
	}

	// Content should be encrypted (base64, not plaintext)
	if evt.Content == message {
		t.Error("content should be encrypted, not plaintext")
	}
	if evt.Content == "" {
		t.Error("content should not be empty")
	}

	// ID should be valid (64-char hex)
	if len(evt.ID) != 64 {
		t.Errorf("expected 64-char hex ID, got %d chars", len(evt.ID))
	}

	// Signature should be non-empty
	if evt.Sig == "" {
		t.Error("expected non-empty signature")
	}

	// Verify we can decrypt the content
	plaintext, err := Decrypt(recipientPriv, senderPriv.PubKey(), evt.Content)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if plaintext != message {
		t.Errorf("decrypted content mismatch: expected %q, got %q", message, plaintext)
	}
}

func TestDecryptDirectMessage(t *testing.T) {
	senderPriv, _ := btcec.NewPrivateKey()
	recipientPriv, _ := btcec.NewPrivateKey()

	senderPrivHex := hex.EncodeToString(senderPriv.Serialize())
	recipientPrivHex := hex.EncodeToString(recipientPriv.Serialize())
	recipientPubHex := hex.EncodeToString(recipientPriv.PubKey().SerializeCompressed())

	message := "Secret DM via NIP-04"

	// Build the event
	evt, err := SendDirectMessage(nil, senderPrivHex, recipientPubHex, message)
	if err != nil {
		t.Fatalf("SendDirectMessage failed: %v", err)
	}

	// Decrypt using DecryptDirectMessage helper
	plaintext, err := DecryptDirectMessage(recipientPrivHex, evt)
	if err != nil {
		t.Fatalf("DecryptDirectMessage failed: %v", err)
	}

	if plaintext != message {
		t.Errorf("decrypted message mismatch: expected %q, got %q", message, plaintext)
	}
}

func TestDecryptDirectMessage_wrongKind(t *testing.T) {
	priv, _ := btcec.NewPrivateKey()
	privHex := hex.EncodeToString(priv.Serialize())

	evt := &Event{
		Kind:    1,
		Content: "not encrypted",
		PubKey:  hex.EncodeToString(priv.PubKey().SerializeCompressed()),
	}

	_, err := DecryptDirectMessage(privHex, evt)
	if err == nil {
		t.Error("expected error for non-kind-4 event, got nil")
	}
}

func TestGetDirectMessageFilters(t *testing.T) {
	myKey := "mypubkeyhex"
	theirKey := "theirpubkeyhex"

	filters := GetDirectMessageFilters(myKey, theirKey)

	if len(filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(filters))
	}

	// Filter 1: messages from them
	f1 := filters[0]
	if len(f1.Kinds) != 1 || f1.Kinds[0] != 4 {
		t.Errorf("filter 1: expected kinds=[4], got %v", f1.Kinds)
	}
	if len(f1.Authors) != 1 || f1.Authors[0] != theirKey {
		t.Errorf("filter 1: expected authors=[%s], got %v", theirKey, f1.Authors)
	}

	// Filter 2: messages from me
	f2 := filters[1]
	if len(f2.Kinds) != 1 || f2.Kinds[0] != 4 {
		t.Errorf("filter 2: expected kinds=[4], got %v", f2.Kinds)
	}
	if len(f2.Authors) != 1 || f2.Authors[0] != myKey {
		t.Errorf("filter 2: expected authors=[%s], got %v", myKey, f2.Authors)
	}
}

func TestPKCS7Padding(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		blockSz  int
		expected int // expected output length
	}{
		{"empty", []byte{}, 16, 16},
		{"1 byte", []byte{1}, 16, 16},
		{"15 bytes", make([]byte, 15), 16, 16},
		{"16 bytes (full block)", make([]byte, 16), 16, 32}, // adds a full padding block
		{"17 bytes", make([]byte, 17), 16, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			padded := pkcs7Pad(tt.input, tt.blockSz)
			if len(padded) != tt.expected {
				t.Errorf("padded length: expected %d, got %d", tt.expected, len(padded))
			}
			if len(padded)%tt.blockSz != 0 {
				t.Errorf("padded length not multiple of block size %d", tt.blockSz)
			}

			unpadded, err := pkcs7Unpad(padded)
			if err != nil {
				t.Fatalf("pkcs7Unpad failed: %v", err)
			}
			if len(unpadded) != len(tt.input) {
				t.Errorf("unpadded length: expected %d, got %d", len(tt.input), len(unpadded))
			}
		})
	}
}

func TestComputeSharedSecret(t *testing.T) {
	// Test that ECDH produces the same shared secret regardless of direction
	priv1, _ := btcec.NewPrivateKey()
	priv2, _ := btcec.NewPrivateKey()
	pub1 := priv1.PubKey()
	pub2 := priv2.PubKey()

	secret12, err := computeSharedSecret(priv1, pub2)
	if err != nil {
		t.Fatalf("computeSharedSecret(1→2) failed: %v", err)
	}

	secret21, err := computeSharedSecret(priv2, pub1)
	if err != nil {
		t.Fatalf("computeSharedSecret(2→1) failed: %v", err)
	}

	if secret12 != secret21 {
		t.Error("ECDH shared secrets should be equal regardless of direction")
	}

	// Different pairs should produce different secrets
	priv3, _ := btcec.NewPrivateKey()
	secret13, _ := computeSharedSecret(priv1, priv3.PubKey())

	if secret12 == secret13 {
		t.Error("shared secrets from different pairs should differ")
	}
}
