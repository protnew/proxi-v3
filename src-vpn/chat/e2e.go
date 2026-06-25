// Package chat implements the messaging layer for Unkillable Messenger.
//
// e2e.go provides End-to-End encryption for direct messages using X25519
// ECDH key agreement, HKDF-based key derivation, and AES-256-GCM encryption.
package chat

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/unkillable-messenger/vpn/crypto"
	"github.com/unkillable-messenger/vpn/store"
)

// E2ESession represents an established End-to-End encryption session between
// two parties. The SharedSecret is derived via X25519 ECDH and HKDF key
// derivation, then used for all subsequent AES-256-GCM encrypt/decrypt
// operations within the session.
type E2ESession struct {
	LocalNpub    string
	RemoteNpub   string
	SharedSecret []byte
	CreatedAt    int64
}

// EncryptMessageForRecipient encrypts a plaintext message for a specific
// recipient using E2E encryption.
//
// The shared secret is derived via X25519 ECDH between the sender's private
// DH key and the recipient's public DH key, then refined with HKDF-SHA256.
// The final encryption uses AES-256-GCM and returns a base64-encoded string
// in wire format: base64(nonce || ciphertext || auth-tag).
//
// Parameters:
//   - senderKey: PreKeyBundle whose IdentityKey field holds the sender's
//     X25519 private key (32 bytes).
//   - recipientPubKey: hex-encoded X25519 public key of the recipient.
func EncryptMessageForRecipient(plaintext string, senderPrivKey []byte, recipientPubKey string) (encrypted string, err error) {
	recipientPub, err := hex.DecodeString(recipientPubKey)
	if err != nil {
		return "", fmt.Errorf("decode recipient public key: %w", err)
	}

	if len(recipientPub) != 32 {
		return "", fmt.Errorf("recipient public key must be 32 bytes, got %d", len(recipientPub))
	}

	// Derive shared secret via X25519 ECDH.
	sharedSecret, err := crypto.DeriveSharedSecret(senderPrivKey, recipientPub)
	if err != nil {
		return "", fmt.Errorf("derive shared secret: %w", err)
	}

	// Derive chat-specific key using HKDF-SHA256.
	var shared [32]byte
	copy(shared[:], sharedSecret)
	chatKey := crypto.DeriveChatKey(shared, "e2e-chat")

	// Encrypt with AES-256-GCM; returns base64(nonce || ciphertext).
	return crypto.EncryptMessage(plaintext, chatKey)
}

// DecryptMessageFromSender decrypts a ciphertext that was encrypted by a
// specific sender using E2E encryption.
//
// The shared secret is derived via X25519 ECDH between the recipient's
// private DH key and the sender's public DH key, producing the same shared
// secret as the sender computed. The message is then decrypted with
// AES-256-GCM.
//
// Parameters:
//   - ciphertext: base64-encoded encrypted message (nonce || ciphertext).
//   - recipientKey: PreKeyBundle whose IdentityKey field holds the recipient's
//     X25519 private key (32 bytes).
//   - senderPubKey: hex-encoded X25519 public key of the sender.
func DecryptMessageFromSender(ciphertext string, recipientPrivKey []byte, senderPubKey string) (plaintext string, err error) {
	senderPub, err := hex.DecodeString(senderPubKey)
	if err != nil {
		return "", fmt.Errorf("decode sender public key: %w", err)
	}

	if len(senderPub) != 32 {
		return "", fmt.Errorf("sender public key must be 32 bytes, got %d", len(senderPub))
	}

	// Derive shared secret via X25519 ECDH (symmetric — same result as sender).
	sharedSecret, err := crypto.DeriveSharedSecret(recipientPrivKey, senderPub)
	if err != nil {
		return "", fmt.Errorf("derive shared secret: %w", err)
	}

	// Derive chat-specific key using HKDF-SHA256 (same context → same key).
	var shared [32]byte
	copy(shared[:], sharedSecret)
	chatKey := crypto.DeriveChatKey(shared, "e2e-chat")

	// Decrypt with AES-256-GCM.
	return crypto.DecryptMessage(ciphertext, chatKey)
}

// GenerateE2ESession creates a new E2E encryption session between two parties
// identified by their Nostr public keys (npubs).
//
// It generates fresh X25519 DH key pairs for both parties, computes the
// shared secret via ECDH, and stores it in the session for subsequent
// encrypt/decrypt operations.
//
// Parameters:
//   - localNpub:  npub of the local (initiating) user.
//   - remoteNpub: npub of the remote (responding) user.
//   - db:         Store for persistence (used for future PreKeyBundle lookups).
func GenerateE2ESession(localNpub, remoteNpub string, db *store.Store) (*E2ESession, error) {
	if localNpub == "" {
		return nil, fmt.Errorf("local npub must not be empty")
	}
	if remoteNpub == "" {
		return nil, fmt.Errorf("remote npub must not be empty")
	}

	// Generate local DH key pair.
	localPriv, _, err := crypto.GenerateDHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate local DH key pair: %w", err)
	}

	// Generate remote DH key pair (in production, the remote public key would
	// be fetched from the remote's PreKeyBundle via the store/network).
	_, remotePub, err := crypto.GenerateDHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate remote DH key pair: %w", err)
	}

	// Compute shared secret via X25519 ECDH.
	sharedSecret, err := crypto.ComputeSharedSecret(localPriv, remotePub)
	if err != nil {
		return nil, fmt.Errorf("compute shared secret: %w", err)
	}

	return &E2ESession{
		LocalNpub:    localNpub,
		RemoteNpub:   remoteNpub,
		SharedSecret: sharedSecret[:],
		CreatedAt:    time.Now().Unix(),
	}, nil
}

// E2ERatchetSession represents an advanced Double Ratchet session for E2E encryption.
// It provides forward secrecy and post-compromise security.
type E2ERatchetSession struct {
	LocalNpub  string
	RemoteNpub string
	Ratchet    *crypto.RatchetState
	CreatedAt  int64
}

// GenerateE2ERatchetSession establishes a Double Ratchet session initialized
// with a shared secret (usually derived via X3DH) and an initiator flag.
func GenerateE2ERatchetSession(localNpub, remoteNpub string, sharedSecret []byte, isInitiator bool) (*E2ERatchetSession, error) {
	if localNpub == "" || remoteNpub == "" {
		return nil, fmt.Errorf("local and remote npub must not be empty")
	}
	if len(sharedSecret) != 32 {
		return nil, fmt.Errorf("shared secret must be 32 bytes")
	}

	ratchet := crypto.NewRatchetState(sharedSecret, isInitiator)

	return &E2ERatchetSession{
		LocalNpub:  localNpub,
		RemoteNpub: remoteNpub,
		Ratchet:    ratchet,
		CreatedAt:  time.Now().Unix(),
	}, nil
}

// EncryptMessage encrypts a plaintext message using the Double Ratchet session.
// It advances the sending chain and returns the ratchet ciphertext.
func (s *E2ERatchetSession) EncryptMessage(plaintext []byte) ([]byte, error) {
	return s.Ratchet.RatchetEncrypt(plaintext)
}

// DecryptMessage decrypts a ciphertext message using the Double Ratchet session.
// It handles out-of-order messages and advances the receiving chain.
func (s *E2ERatchetSession) DecryptMessage(ciphertext []byte) ([]byte, error) {
	return s.Ratchet.RatchetDecrypt(ciphertext)
}
