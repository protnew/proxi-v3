// Package crypto provides E2E encryption primitives for Unkillable Messenger.
//
// dh.go implements X25519 Diffie-Hellman key exchange and HKDF-based key derivation.
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// GenerateDHKeyPair creates an X25519 Diffie-Hellman key pair.
// The private key is a random 32-byte scalar; the public key is derived via curve25519.
func GenerateDHKeyPair() (privateKey [32]byte, publicKey [32]byte, err error) {
	// Generate a random 32-byte private key (clamped internally by X25519).
	if _, err = rand.Read(privateKey[:]); err != nil {
		return [32]byte{}, [32]byte{}, fmt.Errorf("generate private key: %w", err)
	}

	// Derive the public key using curve25519 basepoint multiplication.
	pub, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return [32]byte{}, [32]byte{}, fmt.Errorf("derive public key: %w", err)
	}
	copy(publicKey[:], pub)

	return privateKey, publicKey, nil
}

// ComputeSharedSecret performs X25519 scalar multiplication between a private key
// and a peer's public key, producing a 32-byte shared secret.
func ComputeSharedSecret(privateKey [32]byte, publicKey [32]byte) ([32]byte, error) {
	shared, err := curve25519.X25519(privateKey[:], publicKey[:])
	if err != nil {
		return [32]byte{}, fmt.Errorf("compute shared secret: %w", err)
	}

	var secret [32]byte
	copy(secret[:], shared)
	return secret, nil
}

// DeriveChatKey uses HKDF-SHA256 to derive a 32-byte chat-specific encryption key
// from a shared secret and an application-level context string (e.g. chat ID).
func DeriveChatKey(sharedSecret [32]byte, context string) [32]byte {
	// info param encodes the protocol context — "UnkillableMessenger-E2E" + chat-specific context.
	info := []byte("UnkillableMessenger-E2E:" + context)

	reader := hkdf.New(sha256.New, sharedSecret[:], nil, info)

	var chatKey [32]byte
	// hkdf.Read is guaranteed not to return an error when reading <= hash size (32 bytes).
	if _, err := reader.Read(chatKey[:]); err != nil {
		// This should never happen with a valid HKDF and 32-byte output.
		panic(fmt.Sprintf("crypto: hkdf read failed: %v", err))
	}

	return chatKey
}
