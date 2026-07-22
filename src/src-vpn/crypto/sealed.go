// Package crypto provides E2E encryption primitives for Unkillable Messenger.
//
// sealed.go implements a sealed sender protocol that hides the message sender's
// identity from network observers. The recipient can still identify the sender
// after unsealing.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

const (
	// sealedEphemeralPubSize is the size of the ephemeral public key.
	sealedEphemeralPubSize = 32
	// sealedSenderPubSize is the size of the encrypted sender public key.
	sealedSenderPubSize = 32
	// sealedMinSize is the minimum sealed message size:
	// ephemeral_pub(32) + encrypted_sender_pub(32) + GCM_tag(16)
	sealedMinSize = sealedEphemeralPubSize + sealedSenderPubSize + 16
)

// SealedSender hides the message sender's identity.
type SealedSender struct{}

// SealMessage encrypts a message while hiding the sender's identity from
// everyone except the recipient.
//
// Format: [ephemeral_pub 32bytes][encrypted_sender_pub 32bytes][ciphertext][tag]
//
// The recipient can unseal the message and learn the sender's public key.
func SealMessage(plaintext []byte, senderPriv, recipientPub []byte) ([]byte, error) {
	if len(senderPriv) != 32 {
		return nil, fmt.Errorf("sender private key must be 32 bytes, got %d", len(senderPriv))
	}
	if len(recipientPub) != 32 {
		return nil, fmt.Errorf("recipient public key must be 32 bytes, got %d", len(recipientPub))
	}

	// Derive sender public key from private key
	senderPub, err := curve25519.X25519(senderPriv, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("derive sender public key: %w", err)
	}

	// Generate ephemeral key pair
	var ephemeralPriv [32]byte
	if _, err := rand.Read(ephemeralPriv[:]); err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	ephemeralPub, err := curve25519.X25519(ephemeralPriv[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("derive ephemeral public key: %w", err)
	}

	// ECDH: ephemeral private × recipient public → shared secret
	sharedSecret, err := curve25519.X25519(ephemeralPriv[:], recipientPub)
	if err != nil {
		return nil, fmt.Errorf("compute shared secret: %w", err)
	}

	// Derive encryption key from shared secret using HKDF-like construction
	encKey := deriveSealedKey(sharedSecret, ephemeralPub)

	// Encrypt sender public key with the derived key
	block, err := aes.NewCipher(encKey[:])
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	// Use a nonce derived from the shared secret (deterministic but unique per ephemeral key)
	var nonce [12]byte
	deriveNonce(sharedSecret, nonce[:])

	// Encrypt sender pub as AAD-recoverable data
	// We encrypt both senderPub and plaintext together
	// Payload: senderPub (32 bytes) + plaintext
	payload := make([]byte, 0, len(senderPub)+len(plaintext))
	payload = append(payload, senderPub...)
	payload = append(payload, plaintext...)

	ciphertext := aead.Seal(nil, nonce[:], payload, ephemeralPub)

	// Build final message: ephemeral_pub || encrypted_payload
	result := make([]byte, 0, sealedEphemeralPubSize+len(ciphertext))
	result = append(result, ephemeralPub...)
	result = append(result, ciphertext...)

	return result, nil
}

// UnsealMessage decrypts a sealed message, returning the plaintext and the
// sender's public key.
func UnsealMessage(sealed []byte, recipientPriv []byte) (plaintext []byte, senderPub []byte, err error) {
	if len(recipientPriv) != 32 {
		return nil, nil, fmt.Errorf("recipient private key must be 32 bytes, got %d", len(recipientPriv))
	}
	if len(sealed) < sealedMinSize {
		return nil, nil, fmt.Errorf("sealed message too short: %d bytes", len(sealed))
	}

	// Parse ephemeral public key
	ephemeralPub := sealed[:sealedEphemeralPubSize]
	ciphertext := sealed[sealedEphemeralPubSize:]

	// ECDH: recipient private × ephemeral public → shared secret
	sharedSecret, err := curve25519.X25519(recipientPriv, ephemeralPub)
	if err != nil {
		return nil, nil, fmt.Errorf("compute shared secret: %w", err)
	}

	// Derive the same encryption key
	encKey := deriveSealedKey(sharedSecret, ephemeralPub)

	block, err := aes.NewCipher(encKey[:])
	if err != nil {
		return nil, nil, fmt.Errorf("aes new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize+aead.Overhead() {
		return nil, nil, fmt.Errorf("ciphertext too short")
	}

	var nonce [12]byte
	deriveNonce(sharedSecret, nonce[:])

	// Decrypt with ephemeral pub as additional data
	payload, err := aead.Open(nil, nonce[:], ciphertext, ephemeralPub)
	if err != nil {
		return nil, nil, fmt.Errorf("gcm open: %w", err)
	}

	if len(payload) < 32 {
		return nil, nil, fmt.Errorf("decrypted payload too short to contain sender key")
	}

	// Split sender pub and plaintext
	senderPub = payload[:32]
	plaintext = payload[32:]

	return plaintext, senderPub, nil
}

// deriveSealedKey derives a 32-byte encryption key from the shared secret
// and ephemeral public key using SHA-256.
func deriveSealedKey(sharedSecret, ephemeralPub []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte("UnkillableMessenger-SealedSender-v1"))
	h.Write(sharedSecret)
	h.Write(ephemeralPub)
	return [32]byte(h.Sum(nil))
}

// deriveNonce derives a 12-byte nonce from the shared secret.
func deriveNonce(sharedSecret []byte, nonce []byte) {
	h := sha256.New()
	h.Write([]byte("UnkillableMessenger-SealedSender-Nonce"))
	h.Write(sharedSecret)
	sum := h.Sum(nil)
	copy(nonce, sum[:12])
}
