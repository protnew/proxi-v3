package crypto

import (
	"crypto/rand"
	"testing"
	"time"
)

func TestKeyRotation_Init(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	kr := NewKeyRotation(key)
	if kr == nil {
		t.Fatal("NewKeyRotation returned nil")
	}

	currentKey := kr.CurrentKey()
	if len(currentKey) != 32 {
		t.Errorf("current key length = %d, want 32", len(currentKey))
	}

	rotatedAt := kr.RotatedAt()
	if rotatedAt == 0 {
		t.Error("RotatedAt should not be zero")
	}

	// Should be recent
	now := time.Now().Unix()
	if rotatedAt > now || rotatedAt < now-10 {
		t.Errorf("RotatedAt = %d, expected near %d", rotatedAt, now)
	}
}

func TestKeyRotation_Rotate(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatalf("generate key1: %v", err)
	}
	if _, err := rand.Read(key2); err != nil {
		t.Fatalf("generate key2: %v", err)
	}

	kr := NewKeyRotation(key1)
	initialRotatedAt := kr.RotatedAt()

	// Small delay to ensure timestamp changes
	time.Sleep(10 * time.Millisecond)

	kr.Rotate(key2)

	newRotatedAt := kr.RotatedAt()
	if newRotatedAt < initialRotatedAt {
		t.Errorf("RotatedAt should increase after rotation: before=%d after=%d", initialRotatedAt, newRotatedAt)
	}

	currentKey := kr.CurrentKey()
	if len(currentKey) != 32 {
		t.Errorf("current key length = %d, want 32", len(currentKey))
	}
}

func TestKeyRotation_DecryptWithCurrent(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	kr := NewKeyRotation(key)

	plaintext := []byte("test message for current key")
	ciphertext, err := kr.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := kr.TryDecrypt(ciphertext)
	if err != nil {
		t.Fatalf("TryDecrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestKeyRotation_DecryptWithPrevious(t *testing.T) {
	oldKey := make([]byte, 32)
	newKey := make([]byte, 32)
	if _, err := rand.Read(oldKey); err != nil {
		t.Fatalf("generate old key: %v", err)
	}
	if _, err := rand.Read(newKey); err != nil {
		t.Fatalf("generate new key: %v", err)
	}

	kr := NewKeyRotation(oldKey)

	// Encrypt with old key (before rotation)
	plaintext := []byte("message encrypted with old key")
	ciphertext, err := kr.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt with old key: %v", err)
	}

	// Rotate to new key
	kr.Rotate(newKey)

	// Should still be able to decrypt old ciphertext with previous key
	decrypted, err := kr.TryDecrypt(ciphertext)
	if err != nil {
		t.Fatalf("TryDecrypt with previous key: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}

	// New encryption with current key should also work
	newPlaintext := []byte("message with new key")
	newCiphertext, err := kr.Encrypt(newPlaintext)
	if err != nil {
		t.Fatalf("Encrypt with new key: %v", err)
	}

	decrypted2, err := kr.TryDecrypt(newCiphertext)
	if err != nil {
		t.Fatalf("TryDecrypt new ciphertext: %v", err)
	}
	if string(decrypted2) != string(newPlaintext) {
		t.Errorf("decrypted2 = %q, want %q", decrypted2, newPlaintext)
	}
}

func TestKeyRotation_OldKeyExpired(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key3 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatalf("generate key1: %v", err)
	}
	if _, err := rand.Read(key2); err != nil {
		t.Fatalf("generate key2: %v", err)
	}
	if _, err := rand.Read(key3); err != nil {
		t.Fatalf("generate key3: %v", err)
	}

	kr := NewKeyRotation(key1)

	// Encrypt with key1
	plaintext := []byte("old message from key1")
	ciphertext, err := kr.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt key1: %v", err)
	}

	// Rotate key1 -> key2 (key1 becomes previous)
	kr.Rotate(key2)

	// Rotate key2 -> key3 (key2 becomes previous, key1 is lost)
	kr.Rotate(key3)

	// Now key1 ciphertext should fail (both current=key3 and previous=key2)
	_, err = kr.TryDecrypt(ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with expired key1")
	}
}
