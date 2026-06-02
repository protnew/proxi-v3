// Package nostr implements the Nostr protocol.
//
// nip44.go implements NIP-44 v2 encryption.
// Spec: https://github.com/nostr-protocol/nips/blob/master/44.md
//
// NIP-44 replaces NIP-04's AES-256-CBC with a modern scheme:
//   ECDH(secp256k1) → conversation_key = SHA256(shared_secret || "nip44-v2")
//   encryption      = ChaCha20-Poly1305(conversation_key, nonce, padded_plaintext)
//   wire format     = base64(version[1] + nonce[32] + ciphertext + tag[16])
package nostr

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"golang.org/x/crypto/chacha20poly1305"
)

// NIP-44 version byte
const nip44Version = 0x02

// ==================== Conversation Key ====================

// computeConversationKey derives the NIP-44 v2 conversation key from ECDH.
// conversation_key = SHA256(echd_shared_secret_x || "nip44-v2")
func computeConversationKey(privKey *btcec.PrivateKey, pubKey *btcec.PublicKey) ([32]byte, error) {
	// ECDH: shared_x = privKey * pubKey (x-coordinate)
	sharedX := btcec.GenerateSharedSecret(privKey, pubKey)

	// conversation_key = SHA256(shared_x || "nip44-v2")
	h := sha256.New()
	h.Write(sharedX)
	h.Write([]byte("nip44-v2"))
	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key, nil
}

// ==================== Padding ====================

// nip44Pad applies NIP-44 v2 padding.
// Pads plaintext to next boundary using PKCS#7-style padding.
// Boundary sizes: 32, 64, 128, 256, 512, 1024, 2048, ...
// Minimum padded length is 32 bytes.
// If the input exactly matches a boundary, a full padding block is added.
func nip44Pad(plaintext []byte) []byte {
	plen := len(plaintext)
	if plen < 32 {
		plen = 32
	}
	// Find the next power of 2 that is > plen (always add at least 1 byte of padding)
	// NIP-44 v2 uses levels: 32, 64, 128, 256, 512, 1024, 2048, ...
	boundary := uint(32)
	for boundary <= uint(plen) {
		boundary *= 2
	}
	// PKCS#7 padding
	padLen := int(boundary) - len(plaintext)
	padded := make([]byte, int(boundary))
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}
	return padded
}

// nip44Unpad removes NIP-44 v2 PKCS#7 padding.
func nip44Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("nip44: empty data")
	}
	padVal := int(data[len(data)-1])
	if padVal == 0 || padVal > len(data) {
		return nil, fmt.Errorf("nip44: invalid padding value %d", padVal)
	}
	for i := len(data) - padVal; i < len(data); i++ {
		if data[i] != byte(padVal) {
			return nil, fmt.Errorf("nip44: invalid padding bytes")
		}
	}
	return data[:len(data)-padVal], nil
}

// ==================== Encrypt / Decrypt ====================

// Encrypt44 encrypts plaintext using NIP-44 v2.
//
// Parameters:
//   - senderPrivKey:   sender's secp256k1 private key
//   - recipientPubKey:  recipient's secp256k1 public key
//   - plaintext:        message to encrypt
//
// Wire format: base64(version[1] + nonce[24] + ciphertext_padded + tag[16])
func Encrypt44(senderPrivKey *btcec.PrivateKey, recipientPubKey *btcec.PublicKey, plaintext string) (string, error) {
	// 1. Derive conversation key
	convKey, err := computeConversationKey(senderPrivKey, recipientPubKey)
	if err != nil {
		return "", fmt.Errorf("nip44 encrypt: conversation key: %w", err)
	}

	// 2. Create AEAD cipher (XChaCha20-Poly1305 with 24-byte nonce)
	aead, err := chacha20poly1305.NewX(convKey[:])
	if err != nil {
		return "", fmt.Errorf("nip44 encrypt: create AEAD: %w", err)
	}

	// 3. Generate random nonce (24 bytes for XChaCha20-Poly1305)
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nip44 encrypt: generate nonce: %w", err)
	}

	// 4. Pad plaintext
	padded := nip44Pad([]byte(plaintext))

	// 5. Encrypt: AEAD seals padded plaintext
	// ciphertext includes the 16-byte auth tag appended
	ciphertext := aead.Seal(nil, nonce, padded, nil)

	// 6. Assemble payload: version(1) + nonce(24) + ciphertext+tag
	payload := make([]byte, 0, 1+len(nonce)+len(ciphertext))
	payload = append(payload, nip44Version)
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)

	// 7. Base64 encode
	return base64.StdEncoding.EncodeToString(payload), nil
}

// Decrypt44 decrypts a NIP-44 v2 encrypted message.
//
// Parameters:
//   - recipientPrivKey: recipient's secp256k1 private key
//   - senderPubKey:     sender's secp256k1 public key
//   - ciphertext:       base64-encoded encrypted content
//
// Returns the decrypted plaintext string.
func Decrypt44(recipientPrivKey *btcec.PrivateKey, senderPubKey *btcec.PublicKey, ciphertext string) (string, error) {
	// 1. Decode base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: base64 decode: %w", err)
	}

	// 2. Validate minimum length: version(1) + nonce(24) + tag(16) = 41 bytes minimum
	if len(data) < 1+24+16 {
		return "", fmt.Errorf("nip44 decrypt: payload too short (%d bytes)", len(data))
	}

	// 3. Check version byte
	if data[0] != nip44Version {
		return "", fmt.Errorf("nip44 decrypt: unsupported version %d, expected %d", data[0], nip44Version)
	}

	// 4. Extract nonce and encrypted data
	nonce := data[1 : 1+24]
	encrypted := data[1+24:]

	// 5. Derive conversation key
	convKey, err := computeConversationKey(recipientPrivKey, senderPubKey)
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: conversation key: %w", err)
	}

	// 6. Create AEAD cipher (XChaCha20-Poly1305 with 24-byte nonce)
	aead, err := chacha20poly1305.NewX(convKey[:])
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: create AEAD: %w", err)
	}

	// 7. Decrypt and verify
	padded, err := aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: authentication failed: %w", err)
	}

	// 8. Remove padding
	plaintext, err := nip44Unpad(padded)
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: %w", err)
	}

	return string(plaintext), nil
}
