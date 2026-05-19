package crypto

import (
	"crypto/rand"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	var key [32]byte
	rand.Read(key[:])

	plaintext := []byte("Hello, Unkillable Messenger!")

	ciphertext, nonce, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Ciphertext should differ from plaintext
	if string(ciphertext) == string(plaintext) {
		t.Error("ciphertext should not equal plaintext")
	}

	// Decrypt should recover original
	decrypted, err := Decrypt(ciphertext, nonce, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted mismatch: got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestEncryptMessageDecryptMessage(t *testing.T) {
	var key [32]byte
	rand.Read(key[:])

	original := "This is a secret message for E2E testing 🚀"

	encrypted, err := EncryptMessage(original, key)
	if err != nil {
		t.Fatalf("EncryptMessage: %v", err)
	}

	// Encrypted should be base64 and different from original
	if encrypted == original {
		t.Error("encrypted should differ from original")
	}

	decrypted, err := DecryptMessage(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptMessage: %v", err)
	}
	if decrypted != original {
		t.Errorf("roundtrip failed: got %q, want %q", decrypted, original)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	var key1, key2 [32]byte
	rand.Read(key1[:])
	rand.Read(key2[:])

	encrypted, err := EncryptMessage("test message", key1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecryptMessage(encrypted, key2)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	var key [32]byte
	rand.Read(key[:])

	encrypted, err := EncryptMessage("", key)
	if err != nil {
		t.Fatalf("EncryptMessage(empty): %v", err)
	}

	decrypted, err := DecryptMessage(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptMessage(empty): %v", err)
	}
	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}
