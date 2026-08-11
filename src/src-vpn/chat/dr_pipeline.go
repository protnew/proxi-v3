// Package chat — DR pipeline wires Double Ratchet into message send/receive.
// CRYP-010: encrypt BEFORE persist; SQLite holds ciphertext only.
package chat

import (
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/unkillable-messenger/vpn/crypto"
)

// DRSessionStore keeps in-memory Double Ratchet sessions keyed by conversation pair.
// Not durable across restarts — production must persist ratchet state (follow-up).
type DRSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*crypto.RatchetState // key = pairKey(a,b) from local perspective: local|remote
}

// NewDRSessionStore creates an empty session store.
func NewDRSessionStore() *DRSessionStore {
	return &DRSessionStore{sessions: make(map[string]*crypto.RatchetState)}
}

func pairKey(local, remote string) string {
	return local + "|" + remote
}

// PutSession stores a ratchet state for local→remote conversation.
func (s *DRSessionStore) PutSession(local, remote string, rs *crypto.RatchetState) {
	if s == nil || rs == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[pairKey(local, remote)] = rs
}

// GetSession returns ratchet state if present.
func (s *DRSessionStore) GetSession(local, remote string) (*crypto.RatchetState, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rs, ok := s.sessions[pairKey(local, remote)]
	return rs, ok
}

// HasSession reports whether a DR session exists.
func (s *DRSessionStore) HasSession(local, remote string) bool {
	_, ok := s.GetSession(local, remote)
	return ok
}

// BootstrapPair runs X3DH+DR and stores both Alice and Bob ratchet states.
// Used when both parties' material is available (tests + desktop key ceremony).
func (s *DRSessionStore) BootstrapPair(aliceNpub, bobNpub string, aliceIK crypto.X3DHKeyPair, bob *crypto.X3DHBobMaterial) error {
	if s == nil {
		return fmt.Errorf("nil DRSessionStore")
	}
	aliceRS, bobRS, err := crypto.BootstrapDoubleRatchet(aliceIK, bob)
	if err != nil {
		return err
	}
	s.PutSession(aliceNpub, bobNpub, aliceRS)
	s.PutSession(bobNpub, aliceNpub, bobRS)
	return nil
}

// EncryptOutbound encrypts plaintext with DR for local→remote.
// Returns base64 wire ciphertext and true if DR session used.
func (s *DRSessionStore) EncryptOutbound(local, remote, plaintext string) (ciphertext string, usedDR bool, err error) {
	rs, ok := s.GetSession(local, remote)
	if !ok {
		return plaintext, false, nil
	}
	ct, err := rs.RatchetEncrypt([]byte(plaintext))
	if err != nil {
		return "", true, fmt.Errorf("DR encrypt: %w", err)
	}
	return "dr1:" + base64.StdEncoding.EncodeToString(ct), true, nil
}

// DecryptInbound decrypts DR ciphertext for local receiving from remote.
// If not DR format, returns input unchanged.
func (s *DRSessionStore) DecryptInbound(local, remote, payload string) (plaintext string, usedDR bool, err error) {
	const prefix = "dr1:"
	if len(payload) < len(prefix) || payload[:len(prefix)] != prefix {
		return payload, false, nil
	}
	rs, ok := s.GetSession(local, remote)
	if !ok {
		return "", true, fmt.Errorf("DR session missing for %s←%s", local, remote)
	}
	raw, err := base64.StdEncoding.DecodeString(payload[len(prefix):])
	if err != nil {
		return "", true, fmt.Errorf("DR b64: %w", err)
	}
	pt, err := rs.RatchetDecrypt(raw)
	if err != nil {
		return "", true, fmt.Errorf("DR decrypt: %w", err)
	}
	return string(pt), true, nil
}

// IsDRCiphertext reports dr1: prefix.
func IsDRCiphertext(s string) bool {
	return len(s) >= 4 && s[:4] == "dr1:"
}


// CreateSessionPair runs full X3DH + Double Ratchet bootstrap (CRYP-011).
// Preferred API when both parties' material is available on this node.
func (s *DRSessionStore) CreateSessionPair(local, remote string, aliceIK crypto.X3DHKeyPair, bob *crypto.X3DHBobMaterial) error {
	return s.BootstrapPair(local, remote, aliceIK, bob)
}

// CreateSessionAsInitiator runs X3DH Initiate (CRYP-011) and stores Alice DR state.
// For a working encrypt/decrypt pair, prefer CreateSessionPair / BootstrapPair
// so both RemotePub values are cross-linked.
func (s *DRSessionStore) CreateSessionAsInitiator(local, remote string, aliceIK crypto.X3DHKeyPair, bobPub *crypto.X3DHBobMaterial) (ekPub [32]byte, err error) {
	if s == nil {
		return ekPub, fmt.Errorf("nil store")
	}
	out, err := crypto.X3DHInitiateAlice(aliceIK, bobPub)
	if err != nil {
		return ekPub, fmt.Errorf("X3DH initiate: %w", err)
	}
	// Also create Bob side so sessions are immediately usable (same node ceremony).
	skB, err := crypto.X3DHRespondBob(bobPub, out.IKPub, out.EKPub)
	if err != nil {
		return ekPub, fmt.Errorf("X3DH respond: %w", err)
	}
	if out.SharedSecret != skB {
		return ekPub, fmt.Errorf("x3dh shared secret mismatch")
	}
	aliceRS := crypto.NewRatchetState(out.SharedSecret[:], true)
	bobRS := crypto.NewRatchetState(skB[:], false)
	aliceRS.RemotePub = bobRS.DHPub
	bobRS.RemotePub = aliceRS.DHPub
	s.PutSession(local, remote, aliceRS)
	s.PutSession(remote, local, bobRS)
	return out.EKPub, nil
}

// CreateSessionAsResponder is kept for API symmetry; pair is usually created in Initiate.
func (s *DRSessionStore) CreateSessionAsResponder(local, remote string, bob *crypto.X3DHBobMaterial, aliceIKPub, aliceEKPub [32]byte) error {
	if s == nil {
		return fmt.Errorf("nil store")
	}
	if s.HasSession(local, remote) {
		return nil // already established via initiator path
	}
	sk, err := crypto.X3DHRespondBob(bob, aliceIKPub, aliceEKPub)
	if err != nil {
		return fmt.Errorf("X3DH respond: %w", err)
	}
	rs := crypto.NewRatchetState(sk[:], false)
	rs.RemotePub = aliceEKPub
	s.PutSession(local, remote, rs)
	return nil
}
