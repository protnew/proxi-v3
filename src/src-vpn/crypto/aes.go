// Package crypto provides E2E encryption primitives for Unkillable Messenger.
//
// aes.go implements AES-256-GCM authenticated encryption with two layers:
//   - low-level Encrypt/Decrypt working with raw byte slices and nonces
//   - high-level EncryptMessage/DecryptMessage with base64 wire encoding
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// Encrypt performs AES-256-GCM encryption on plaintext using the given 32-byte key.
// It returns the ciphertext (with GCM auth tag appended) and the randomly generated 12-byte nonce.
func Encrypt(plaintext []byte, key [32]byte) (ciphertext []byte, nonce [12]byte, err error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, [12]byte{}, fmt.Errorf("aes.NewCipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, [12]byte{}, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	// Generate a random nonce.
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, [12]byte{}, fmt.Errorf("generate nonce: %w", err)
	}

	// Seal appends the ciphertext and auth tag to the nonce prefix; we extract just the ciphertext.
	ciphertext = aead.Seal(nil, nonce[:], plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt performs AES-256-GCM decryption on ciphertext using the given 32-byte key
// and 12-byte nonce. It returns the authenticated plaintext.
func Decrypt(ciphertext []byte, nonce [12]byte, key [32]byte) (plaintext []byte, err error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	plaintext, err = aead.Open(nil, nonce[:], ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}

	return plaintext, nil
}

// EncryptMessage encrypts a text string and returns a base64-encoded string.
// Wire format: base64(nonce || ciphertext) — the 12-byte nonce is prepended to the
// ciphertext before encoding so the receiver can split them deterministically.
func EncryptMessage(text string, key [32]byte) (encrypted string, err error) {
	ciphertext, nonce, err := Encrypt([]byte(text), key)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	// Combine nonce + ciphertext for wire transport.
	combined := make([]byte, 0, len(nonce)+len(ciphertext))
	combined = append(combined, nonce[:]...)
	combined = append(combined, ciphertext...)

	return base64.StdEncoding.EncodeToString(combined), nil
}

// DecryptMessage decodes a base64-encoded message and decrypts it.
// It expects the wire format produced by EncryptMessage: base64(nonce || ciphertext).
func DecryptMessage(encrypted string, key [32]byte) (text string, err error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	if len(data) < 12 {
		return "", fmt.Errorf("message too short: expected at least 12 bytes (nonce), got %d", len(data))
	}

	// Split nonce and ciphertext.
	var nonce [12]byte
	copy(nonce[:], data[:12])
	ciphertext := data[12:]

	plaintext, err := Decrypt(ciphertext, nonce, key)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
