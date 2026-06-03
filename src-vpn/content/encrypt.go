package content

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// GenerateMasterKey generates a cryptographically random 32-byte AES-256 key.
func GenerateMasterKey() ([32]byte, error) {
	var key [32]byte
	if _, err := io.ReadFull(rand.Reader, key[:]); err != nil {
		return [32]byte{}, fmt.Errorf("generate master key: %w", err)
	}
	return key, nil
}

// EncryptedChunk is the wire format for an encrypted chunk.
// Nonce (12 bytes) is prepended to the ciphertext so the receiver can
// split them deterministically.
type EncryptedChunk struct {
	ID    string // original chunk ID (hash hex)
	Index int
	Data  []byte // nonce || ciphertext
}

// encryptAES256GCM is a standalone AES-256-GCM seal (no dependency on crypto/ pkg).
func encryptAES256GCM(key [32]byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	// Output: nonce || ciphertext+tag
	ct := aead.Seal(nonce, nonce, plaintext, nil)
	return ct, nil
}

// decryptAES256GCM is a standalone AES-256-GCM open.
func decryptAES256GCM(key [32]byte, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize+aead.Overhead() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := data[:nonceSize], data[nonceSize:]
	plain, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plain, nil
}

// EncryptChunk encrypts a Chunk with AES-256-GCM and returns an EncryptedChunk.
func EncryptChunk(key [32]byte, c Chunk) (EncryptedChunk, error) {
	ct, err := encryptAES256GCM(key, c.Data)
	if err != nil {
		return EncryptedChunk{}, fmt.Errorf("encrypt chunk %d: %w", c.Index, err)
	}
	return EncryptedChunk{
		ID:    c.ID,
		Index: c.Index,
		Data:  ct,
	}, nil
}

// DecryptChunk decrypts an EncryptedChunk back to a Chunk.
func DecryptChunk(key [32]byte, ec EncryptedChunk) (Chunk, error) {
	plain, err := decryptAES256GCM(key, ec.Data)
	if err != nil {
		return Chunk{}, fmt.Errorf("decrypt chunk %d: %w", ec.Index, err)
	}
	chunk := Chunk{
		ID:    ec.ID,
		Index: ec.Index,
		Data:  plain,
	}
	chunk.computeHash()
	return chunk, nil
}

// EncryptManifest encrypts arbitrary manifest data with AES-256-GCM.
// Returns nonce || ciphertext.
func EncryptManifest(key [32]byte, data []byte) ([]byte, error) {
	return encryptAES256GCM(key, data)
}

// DecryptManifest decrypts manifest data produced by EncryptManifest.
func DecryptManifest(key [32]byte, data []byte) ([]byte, error) {
	return decryptAES256GCM(key, data)
}
