// Package crypto provides E2E encryption primitives for Unkillable Messenger.
//
// keyexchange.go implements Signal-style prekey bundles, X25519 ECDH shared
// secret derivation, and AES-256-GCM peer encryption/decryption.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// PreKeyBundle represents a Signal-style prekey bundle for E2E key exchange.
type PreKeyBundle struct {
	IdentityKey  []byte `json:"identity_key"`
	SignedPreKey []byte `json:"signed_prekey"`
	Signature    []byte `json:"signature"`
	OneTimePreKey []byte `json:"one_time_prekey"`
}

// GeneratePreKeyBundle generates a prekey bundle from an Ed25519 identity key.
// It creates a signed X25519 prekey and a one-time prekey.
func GeneratePreKeyBundle(identityKey []byte) (*PreKeyBundle, error) {
	if len(identityKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("identity key must be %d bytes (Ed25519 private key), got %d", ed25519.PrivateKeySize, len(identityKey))
	}

	// Generate a signed prekey (X25519)
	_, signedPreKeyPub, err := GenerateDHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate signed prekey: %w", err)
	}

	// Sign the signed prekey with the identity key
	signature := ed25519.Sign(identityKey, signedPreKeyPub[:])

	// Generate a one-time prekey (X25519)
	_, oneTimePreKeyPub, err := GenerateDHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate one-time prekey: %w", err)
	}

	// Extract public portion of identity key for the bundle
	identityPub := identityKey[ed25519.PrivateKeySize-ed25519.PublicKeySize:]

	return &PreKeyBundle{
		IdentityKey:   identityPub,
		SignedPreKey:  signedPreKeyPub[:],
		Signature:     signature,
		OneTimePreKey: oneTimePreKeyPub[:],
	}, nil
}

// VerifyPreKeyBundle verifies the signature in a prekey bundle against the
// identity key. Returns true if the signature is valid.
func VerifyPreKeyBundle(bundle *PreKeyBundle) bool {
	if bundle == nil || len(bundle.IdentityKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(bundle.IdentityKey, bundle.SignedPreKey, bundle.Signature)
}

// DeriveSharedSecret performs X25519 ECDH between a private key and a peer's
// public key to produce a 32-byte shared secret.
func DeriveSharedSecret(myPrivate, theirPublic []byte) ([]byte, error) {
	if len(myPrivate) != 32 {
		return nil, fmt.Errorf("private key must be 32 bytes, got %d", len(myPrivate))
	}
	if len(theirPublic) != 32 {
		return nil, fmt.Errorf("public key must be 32 bytes, got %d", len(theirPublic))
	}

	// Clamp the private key for X25519 (as done in curve25519)
	var priv [32]byte
	copy(priv[:], myPrivate)

	shared, err := curve25519.X25519(priv[:], theirPublic)
	if err != nil {
		return nil, fmt.Errorf("x25519 scalar multiplication: %w", err)
	}

	return shared, nil
}

// EncryptForPeer encrypts plaintext using AES-256-GCM with the given shared
// secret. The output format is: nonce (12 bytes) || ciphertext || auth tag.
func EncryptForPeer(sharedSecret []byte, plaintext []byte) ([]byte, error) {
	if len(sharedSecret) != 32 {
		return nil, fmt.Errorf("shared secret must be 32 bytes, got %d", len(sharedSecret))
	}

	block, err := aes.NewCipher(sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce
	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptFromPeer decrypts ciphertext produced by EncryptForPeer using
// AES-256-GCM with the given shared secret.
func DecryptFromPeer(sharedSecret []byte, ciphertext []byte) ([]byte, error) {
	if len(sharedSecret) != 32 {
		return nil, fmt.Errorf("shared secret must be 32 bytes, got %d", len(sharedSecret))
	}

	block, err := aes.NewCipher(sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize+aead.Overhead() {
		return nil, fmt.Errorf("ciphertext too short: %d bytes", len(ciphertext))
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}

	return plaintext, nil
}

// privateKeyToCurve25519 converts an Ed25519 private key to a Curve25519
// private key using the method from RFC 8032.
func privateKeyToCurve25519(priv ed25519.PrivateKey) *[32]byte {
	// Hash the Ed25519 private key seed
	h := sha512.New()
	h.Write(priv[:32])
	var digest [64]byte
	copy(digest[:], h.Sum(nil))

	// Clamp
	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64

	var out [32]byte
	copy(out[:], digest[:32])
	return &out
}
