package nostr

import (
	"encoding/base64"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
)

// ==================== NIP-44 v2 Encrypt/Decrypt Tests ====================

func TestEncrypt44Decrypt44(t *testing.T) {
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
		{"short", "Hi"},
		{"exactly 32 bytes", "1234567890123456789012345678901"},
		{"31 bytes", "123456789012345678901234567890"},
		{"33 bytes", "12345678901234567890123456789012"},
		{"long message", "This is a much longer message that spans multiple padding boundaries and tests that the ChaCha20-Poly1305 encryption handles larger payloads correctly. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt: sender encrypts for recipient
			ciphertext, err := Encrypt44(senderPriv, recipientPub, tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt44 failed: %v", err)
			}

			if ciphertext == "" && tt.plaintext != "" {
				t.Error("expected non-empty ciphertext")
			}

			// Decrypt: recipient decrypts from sender
			plaintext, err := Decrypt44(recipientPriv, senderPub, ciphertext)
			if err != nil {
				t.Fatalf("Decrypt44 failed: %v", err)
			}

			if plaintext != tt.plaintext {
				t.Errorf("plaintext mismatch:\nexpected: %q\n got: %q", tt.plaintext, plaintext)
			}
		})
	}
}

func TestEncrypt44Decrypt44Reverse(t *testing.T) {
	// Test that the recipient can also encrypt and the sender can decrypt
	senderPriv, _ := btcec.NewPrivateKey()
	recipientPriv, _ := btcec.NewPrivateKey()
	senderPub := senderPriv.PubKey()
	recipientPub := recipientPriv.PubKey()

	message := "Reply from recipient via NIP-44"

	// Recipient encrypts for sender
	ciphertext, err := Encrypt44(recipientPriv, senderPub, message)
	if err != nil {
		t.Fatalf("Encrypt44 (reverse) failed: %v", err)
	}

	// Sender decrypts from recipient
	plaintext, err := Decrypt44(senderPriv, recipientPub, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt44 (reverse) failed: %v", err)
	}

	if plaintext != message {
		t.Errorf("reverse decrypt mismatch: expected %q, got %q", message, plaintext)
	}
}

func TestEncrypt44Decrypt44DifferentKeys_fails(t *testing.T) {
	senderPriv, _ := btcec.NewPrivateKey()
	recipientPriv, _ := btcec.NewPrivateKey()
	eavesdropperPriv, _ := btcec.NewPrivateKey()

	recipientPub := recipientPriv.PubKey()
	eavesdropperPub := eavesdropperPriv.PubKey()

	message := "Secret NIP-44 message"

	// Sender encrypts for the recipient
	ciphertext, err := Encrypt44(senderPriv, recipientPub, message)
	if err != nil {
		t.Fatalf("Encrypt44 failed: %v", err)
	}

	// Eavesdropper tries to decrypt with their own private key — should fail
	_, err = Decrypt44(eavesdropperPriv, senderPriv.PubKey(), ciphertext)
	if err == nil {
		t.Error("expected decryption to fail with wrong private key, but it succeeded")
	}

	// Eavesdropper tries to decrypt with the right private key but wrong sender pubkey — should fail
	_, err = Decrypt44(recipientPriv, eavesdropperPub, ciphertext)
	if err == nil {
		t.Error("expected decryption to fail with wrong sender public key, but it succeeded")
	}

	// Correct decryption should still work
	plaintext, err := Decrypt44(recipientPriv, senderPriv.PubKey(), ciphertext)
	if err != nil {
		t.Fatalf("correct Decrypt44 failed: %v", err)
	}
	if plaintext != message {
		t.Errorf("plaintext mismatch: expected %q, got %q", message, plaintext)
	}
}

func TestEncrypt44ProducesDifferentCiphertexts(t *testing.T) {
	// Encrypting the same message twice should produce different ciphertexts
	// because the nonce is random
	senderPriv, _ := btcec.NewPrivateKey()
	recipientPriv, _ := btcec.NewPrivateKey()
	recipientPub := recipientPriv.PubKey()

	message := "Same NIP-44 message"

	ct1, err := Encrypt44(senderPriv, recipientPub, message)
	if err != nil {
		t.Fatalf("Encrypt44 1 failed: %v", err)
	}
	ct2, err := Encrypt44(senderPriv, recipientPub, message)
	if err != nil {
		t.Fatalf("Encrypt44 2 failed: %v", err)
	}

	if ct1 == ct2 {
		t.Error("expected different ciphertexts due to random nonce, but got identical")
	}

	// Both should decrypt to the same plaintext
	pt1, err := Decrypt44(recipientPriv, senderPriv.PubKey(), ct1)
	if err != nil {
		t.Fatalf("Decrypt44 1 failed: %v", err)
	}
	pt2, err := Decrypt44(recipientPriv, senderPriv.PubKey(), ct2)
	if err != nil {
		t.Fatalf("Decrypt44 2 failed: %v", err)
	}

	if pt1 != message || pt2 != message {
		t.Errorf("decrypted messages don't match original: %q, %q", pt1, pt2)
	}
}

func TestDecrypt44InvalidCiphertext(t *testing.T) {
	priv1, _ := btcec.NewPrivateKey()
	priv2, _ := btcec.NewPrivateKey()
	pub2 := priv2.PubKey()

	tests := []struct {
		name       string
		ciphertext string
	}{
		{"empty string", ""},
		{"not base64", "!!!not-base64!!!"},
		{"too short", "AQI="},
		{"valid base64 but wrong version", "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt44(priv1, pub2, tt.ciphertext)
			if err == nil {
				t.Error("expected error for invalid ciphertext, got nil")
			}
		})
	}
}

func TestEncrypt44ConversationKeyCommutes(t *testing.T) {
	// Test that conversation key derivation is symmetric
	priv1, _ := btcec.NewPrivateKey()
	priv2, _ := btcec.NewPrivateKey()
	pub1 := priv1.PubKey()
	pub2 := priv2.PubKey()

	key12, err := computeConversationKey(priv1, pub2)
	if err != nil {
		t.Fatalf("computeConversationKey(1→2) failed: %v", err)
	}

	key21, err := computeConversationKey(priv2, pub1)
	if err != nil {
		t.Fatalf("computeConversationKey(2→1) failed: %v", err)
	}

	if key12 != key21 {
		t.Error("conversation keys should be equal regardless of direction")
	}

	// Different pairs should produce different keys
	priv3, _ := btcec.NewPrivateKey()
	key13, _ := computeConversationKey(priv1, priv3.PubKey())

	if key12 == key13 {
		t.Error("conversation keys from different pairs should differ")
	}
}

func TestNip44Padding(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int // expected padded length
	}{
		{"empty", []byte{}, 64},            // plen=32, boundary loops to 64
		{"1 byte", []byte{1}, 64},          // plen=32, boundary loops to 64
		{"31 bytes", make([]byte, 31), 64}, // plen=32, boundary loops to 64
		{"32 bytes", make([]byte, 32), 64}, // boundary 32 <= 32, goes to 64
		{"33 bytes", make([]byte, 33), 64},
		{"64 bytes", make([]byte, 64), 128}, // boundary 64 <= 64, goes to 128
		{"65 bytes", make([]byte, 65), 128},
		{"128 bytes", make([]byte, 128), 256}, // boundary 128 <= 128, goes to 256
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			padded := nip44Pad(tt.input)
			if len(padded) != tt.expected {
				t.Errorf("padded length: expected %d, got %d", tt.expected, len(padded))
			}

			unpadded, err := nip44Unpad(padded)
			if err != nil {
				t.Fatalf("nip44Unpad failed: %v", err)
			}
			if len(unpadded) != len(tt.input) {
				t.Errorf("unpadded length: expected %d, got %d", len(tt.input), len(unpadded))
			}
		})
	}
}

func TestEncrypt44VersionByte(t *testing.T) {
	// Verify that the version byte in the encrypted output is 0x02
	senderPriv, _ := btcec.NewPrivateKey()
	recipientPriv, _ := btcec.NewPrivateKey()

	ct, err := Encrypt44(senderPriv, recipientPriv.PubKey(), "test")
	if err != nil {
		t.Fatalf("Encrypt44 failed: %v", err)
	}

	// Decode base64 and check first byte
	data, _ := base64.StdEncoding.DecodeString(ct)
	if data[0] != 0x02 {
		t.Errorf("expected version byte 0x02, got 0x%02x", data[0])
	}
}
