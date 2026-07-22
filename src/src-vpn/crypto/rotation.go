// Package crypto provides E2E encryption primitives for Unkillable Messenger.
//
// rotation.go implements automatic key rotation with fallback to the previous
// key for decryption during the transition window.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// KeyRotation manages a current and previous encryption key, allowing
// decryption with both during a rotation transition window.
type KeyRotation struct {
	mu          sync.RWMutex
	currentKey  []byte
	previousKey []byte
	rotatedAt   int64
}

// NewKeyRotation creates a new KeyRotation manager with the given initial key.
// The key must be 32 bytes (AES-256).
func NewKeyRotation(initialKey []byte) *KeyRotation {
	key := make([]byte, len(initialKey))
	copy(key, initialKey)
	return &KeyRotation{
		currentKey: key,
		rotatedAt:  time.Now().Unix(),
	}
}

// Rotate replaces the current key with newKey. The old current key becomes
// the previous key, enabling decryption of messages encrypted with either key.
func (kr *KeyRotation) Rotate(newKey []byte) {
	kr.mu.Lock()
	defer kr.mu.Unlock()

	// Move current to previous
	if kr.previousKey != nil {
		// Clear old previous key for security
		for i := range kr.previousKey {
			kr.previousKey[i] = 0
		}
	}
	kr.previousKey = kr.currentKey

	// Set new current key
	key := make([]byte, len(newKey))
	copy(key, newKey)
	kr.currentKey = key
	kr.rotatedAt = time.Now().Unix()
}

// CurrentKey returns a copy of the current encryption key.
func (kr *KeyRotation) CurrentKey() []byte {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	key := make([]byte, len(kr.currentKey))
	copy(key, kr.currentKey)
	return key
}

// RotatedAt returns the Unix timestamp of the last key rotation.
func (kr *KeyRotation) RotatedAt() int64 {
	kr.mu.RLock()
	defer kr.mu.RUnlock()
	return kr.rotatedAt
}

// TryDecrypt attempts to decrypt ciphertext using the current key first,
// then falls back to the previous key if available.
//
// The ciphertext format is: nonce(12) || encrypted_data
func (kr *KeyRotation) TryDecrypt(ciphertext []byte) ([]byte, error) {
	// Try current key first
	plaintext, err := kr.tryDecryptWithKey(kr.currentKey, ciphertext)
	if err == nil {
		return plaintext, nil
	}

	// Fall back to previous key
	kr.mu.RLock()
	prev := kr.previousKey
	kr.mu.RUnlock()

	if prev == nil {
		return nil, fmt.Errorf("decryption failed with current key and no previous key available")
	}

	plaintext, err = kr.tryDecryptWithKey(prev, ciphertext)
	if err == nil {
		return plaintext, nil
	}

	return nil, fmt.Errorf("decryption failed with both current and previous keys")
}

// tryDecryptWithKey attempts AES-GCM decryption with the given key.
func (kr *KeyRotation) tryDecryptWithKey(key, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize+aead.Overhead() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return aead.Open(nil, nonce, ct, nil)
}

// Encrypt encrypts plaintext using the current key.
// Returns: nonce || ciphertext (suitable for TryDecrypt).
func (kr *KeyRotation) Encrypt(plaintext []byte) ([]byte, error) {
	kr.mu.RLock()
	key := kr.currentKey
	kr.mu.RUnlock()

	if len(key) != 32 {
		return nil, fmt.Errorf("current key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Seal appends ciphertext to nonce
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}
