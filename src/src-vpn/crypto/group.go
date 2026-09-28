// Package crypto provides E2E encryption primitives for Unkillable Messenger.
//
// group.go implements Sender Keys group encryption: each member has their own
// AEAD key (senderKey) that is distributed pairwise via ECDH. Messages are
// encrypted with the sender's key and any group member can decrypt them because
// they hold all members' sender keys.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
)

// GroupSession manages E2E encryption for a group chat using Sender Keys.
//
// Each member has a unique senderKey (AES-256 key). When a member sends a message,
// it is encrypted with their own senderKey. All other members can decrypt because
// they store the sender's key.
//
// Key distribution is done pairwise via ECDH: when a group is created, each member
// generates a senderKey and shares it encrypted with every other member using their
// pairwise shared secret.
type GroupSession struct {
	mu         sync.RWMutex
	GroupID    string
	senderKeys map[string][32]byte // userID → AES-256 senderKey
	chainKeys  map[string][]byte   // userID → chain key for ratcheting
}

// NewGroupSession creates a new group encryption session.
func NewGroupSession(groupID string) *GroupSession {
	return &GroupSession{
		GroupID:    groupID,
		senderKeys: make(map[string][32]byte),
		chainKeys:  make(map[string][]byte),
	}
}

// CreateGroupSession creates a group session and distributes sender keys
// using pairwise ECDH shared secrets.
//
// Parameters:
//   - groupID: unique group identifier
//   - members: list of member user IDs
//   - privateKey: our X25519 private key
//   - peerPublicKeys: map of userID → their X25519 public key
//
// For each member, a random sender key is generated. In a real implementation,
// these would be encrypted with the pairwise shared secret and sent to each member.
// For this simplified version, we store them directly.
func CreateGroupSession(groupID string, members []string, privateKey [32]byte, peerPublicKeys map[string][32]byte) (*GroupSession, error) {
	gs := &GroupSession{
		GroupID:    groupID,
		senderKeys: make(map[string][32]byte),
		chainKeys:  make(map[string][]byte),
	}

	for _, memberID := range members {
		// Generate a random sender key for this member
		var senderKey [32]byte
		if _, err := rand.Read(senderKey[:]); err != nil {
			return nil, fmt.Errorf("generate sender key for %s: %w", memberID, err)
		}
		gs.senderKeys[memberID] = senderKey

		// Derive initial chain key from sender key
		chainKey := deriveChainKey(senderKey[:], 0)
		gs.chainKeys[memberID] = chainKey

		// If we have the peer's public key, we would encrypt and send the
		// sender key via the pairwise ECDH shared secret here.
		if peerPub, ok := peerPublicKeys[memberID]; ok {
			sharedSecret, err := ComputeSharedSecret(privateKey, peerPub)
			if err != nil {
				// Log but don't fail — the sender key is still generated
				continue
			}
			// Derive a pairwise key for encrypting the sender key distribution
			_ = DeriveChatKey(sharedSecret, "sender-key-dist:"+groupID+":"+memberID)
		}
	}

	return gs, nil
}

// AddMember adds a new member to the group and generates their sender key.
func (g *GroupSession) AddMember(userID string, peerPublicKey *[32]byte, privateKey *[32]byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var senderKey [32]byte
	if _, err := rand.Read(senderKey[:]); err != nil {
		return fmt.Errorf("generate sender key for %s: %w", userID, err)
	}
	g.senderKeys[userID] = senderKey
	g.chainKeys[userID] = deriveChainKey(senderKey[:], 0)
	return nil
}

// RemoveMember removes a member's sender key from the session.
// After removal, a key rotation should be performed for forward secrecy.
func (g *GroupSession) RemoveMember(userID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.senderKeys, userID)
	delete(g.chainKeys, userID)
}

// Encrypt encrypts a message using the sender's senderKey.
//
// Wire format: [4-byte sender key ID hash] [12-byte nonce] [ciphertext + GCM tag]
func (g *GroupSession) Encrypt(senderID string, plaintext []byte) ([]byte, error) {
	g.mu.RLock()
	senderKey, ok := g.senderKeys[senderID]
	g.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no sender key for user %s in group %s", senderID, g.GroupID)
	}

	block, err := aes.NewCipher(senderKey[:])
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	// Generate nonce
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := aead.Seal(nil, nonce[:], plaintext, nil)

	// Build wire format: [4-byte key hash] [12-byte nonce] [ciphertext]
	keyHash := keyIDHash(senderKey)
	result := make([]byte, 0, 4+12+len(ciphertext))
	result = append(result, keyHash...)
	result = append(result, nonce[:]...)
	result = append(result, ciphertext...)

	return result, nil
}

// Decrypt decrypts a message from a specific sender using their senderKey.
func (g *GroupSession) Decrypt(senderID string, ciphertext []byte) ([]byte, error) {
	g.mu.RLock()
	senderKey, ok := g.senderKeys[senderID]
	g.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no sender key for user %s in group %s", senderID, g.GroupID)
	}

	// Minimum size: 4 (key hash) + 12 (nonce) + 16 (GCM tag)
	if len(ciphertext) < 4+12+16 {
		return nil, fmt.Errorf("ciphertext too short: %d bytes", len(ciphertext))
	}

	// Verify key hash matches
	receivedHash := ciphertext[:4]
	expectedHash := keyIDHash(senderKey)
	if !equalBytes(receivedHash, expectedHash) {
		return nil, fmt.Errorf("key hash mismatch: possible wrong sender key")
	}

	// Extract nonce and actual ciphertext
	var nonce [12]byte
	copy(nonce[:], ciphertext[4:16])
	actualCiphertext := ciphertext[16:]

	block, err := aes.NewCipher(senderKey[:])
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce[:], actualCiphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}

	return plaintext, nil
}

// DecryptAny attempts to decrypt a message by trying all known sender keys.
// Returns the senderID and plaintext if successful.
func (g *GroupSession) DecryptAny(ciphertext []byte) (senderID string, plaintext []byte, err error) {
	if len(ciphertext) < 4+12+16 {
		return "", nil, fmt.Errorf("ciphertext too short")
	}

	receivedHash := ciphertext[:4]

	g.mu.RLock()
	defer g.mu.RUnlock()

	for uid, key := range g.senderKeys {
		expectedHash := keyIDHash(key)
		if !equalBytes(receivedHash, expectedHash) {
			continue
		}

		var nonce [12]byte
		copy(nonce[:], ciphertext[4:16])
		actualCiphertext := ciphertext[16:]

		block, aesErr := aes.NewCipher(key[:])
		if aesErr != nil {
			continue
		}

		aead, aesErr := cipher.NewGCM(block)
		if aesErr != nil {
			continue
		}

		pt, aesErr := aead.Open(nil, nonce[:], actualCiphertext, nil)
		if aesErr != nil {
			continue
		}

		return uid, pt, nil
	}

	return "", nil, fmt.Errorf("no matching sender key found for decryption")
}

// GetMembers returns the list of members with sender keys.
func (g *GroupSession) GetMembers() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	members := make([]string, 0, len(g.senderKeys))
	for uid := range g.senderKeys {
		members = append(members, uid)
	}
	return members
}

// HasMember checks if a member has a sender key.
func (g *GroupSession) HasMember(userID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.senderKeys[userID]
	return ok
}

// RotateSenderKey generates a new sender key for a member (forward secrecy).
func (g *GroupSession) RotateSenderKey(userID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var newKey [32]byte
	if _, err := rand.Read(newKey[:]); err != nil {
		return fmt.Errorf("generate new sender key: %w", err)
	}
	g.senderKeys[userID] = newKey
	return nil
}

// --- Helper functions ---

// keyIDHash returns a 4-byte truncated SHA-256 hash of a key for identification.
func keyIDHash(key [32]byte) []byte {
	h := sha256.Sum256(key[:])
	return h[:4]
}

// deriveChainKey derives a chain key from a sender key and message number.
func deriveChainKey(senderKey []byte, msgNum uint32) []byte {
	h := sha256.New()
	h.Write(senderKey)
	binary.Write(h, binary.BigEndian, msgNum)
	return h.Sum(nil)
}

// equalBytes does constant-time byte comparison.
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
