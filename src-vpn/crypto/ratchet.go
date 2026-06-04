// Package crypto provides E2E encryption primitives for Unkillable Messenger.
//
// ratchet.go implements the Double Ratchet algorithm for forward-secret
// messaging. Each party maintains a RatchetState with root key, sending and
// receiving chain keys, and a skipped-key cache for out-of-order messages.
package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"sync"
)

// MaxSkippedKeys is the upper bound on the number of skipped message keys
// kept in memory. Older entries are evicted when the limit is exceeded.
const MaxSkippedKeys = 40

// skippedKeyEntry identifies a skipped message key by (remoteDH, msgNum).
type skippedKeyEntry struct {
	dhPub  [32]byte
	msgNum uint32
}

// RatchetState holds the full double-ratchet state for one party.
type RatchetState struct {
	mu sync.Mutex

	RootKey      [32]byte
	SendChainKey [32]byte
	RecvChainKey [32]byte
	SendNum      uint32 // message number in current sending chain
	RecvNum      uint32 // message number in current receiving chain

	DHPriv    [32]byte
	DHPub     [32]byte
	RemotePub [32]byte

	SkippedKeys map[skippedKeyEntry][32]byte
}

// ratchetHeader is prepended to every ciphertext so the receiver can
// identify which DH step and chain position the message belongs to.
type ratchetHeader struct {
	DHPub  [32]byte
	SendNum uint32
	PrevNum uint32
}

// NewRatchetState creates a double-ratchet state from a shared secret.
// The initiator performs the first DH ratchet step immediately.
func NewRatchetState(sharedSecret []byte, isInitiator bool) *RatchetState {
	var rootKey [32]byte
	copy(rootKey[:], sharedSecret)

	rs := &RatchetState{
		RootKey:     rootKey,
		SendNum:     0,
		RecvNum:     0,
		SkippedKeys: make(map[skippedKeyEntry][32]byte),
	}

	// Generate our DH key pair
	priv, pub, err := GenerateDHKeyPair()
	if err != nil {
		panic("ratchet: generate DH key pair: " + err.Error())
	}
	rs.DHPriv = priv
	rs.DHPub = pub

	// Derive initial chain keys from the root key using HKDF-like split
	// rootKey → (newRootKey, sendChain, recvChain)
	// We use three different info strings to derive three independent keys.
	if isInitiator {
		rs.SendChainKey = deriveRootChain(rs.RootKey, []byte("RatchetSend"))
		rs.RecvChainKey = deriveRootChain(rs.RootKey, []byte("RatchetRecv"))
	} else {
		rs.SendChainKey = deriveRootChain(rs.RootKey, []byte("RatchetRecv"))
		rs.RecvChainKey = deriveRootChain(rs.RootKey, []byte("RatchetSend"))
	}

	return rs
}

// RatchetEncrypt encrypts plaintext using the double ratchet.
// It advances the sending chain key and encrypts with the message key.
// Wire format: headerLen(2) || header || nonce(12) || ciphertext+tag
func (rs *RatchetState) RatchetEncrypt(plaintext []byte) ([]byte, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Derive message key from current send chain key, then advance chain
	msgKey := advanceChainKey(&rs.SendChainKey)

	// Build header
	hdr := ratchetHeader{
		DHPub:   rs.DHPub,
		SendNum: rs.SendNum,
		PrevNum: rs.RecvNum,
	}
	rs.SendNum++

	// Serialize header: DHPub(32) + SendNum(4) + PrevNum(4) = 40 bytes
	hdrBytes := make([]byte, 40)
	copy(hdrBytes[0:32], hdr.DHPub[:])
	putUint32(hdrBytes[32:36], hdr.SendNum)
	putUint32(hdrBytes[36:40], hdr.PrevNum)

	// Encrypt with AES-256-GCM using the message key
	var mk [32]byte
	copy(mk[:], msgKey[:])

	// Use AES-256-GCM for encryption
	ct, nonce, err := Encrypt(plaintext, mk)
	if err != nil {
		return nil, fmt.Errorf("ratchet encrypt: %w", err)
	}

	// Wire format: headerLen(2) || header || nonce(12) || ciphertext
	out := make([]byte, 0, 2+len(hdrBytes)+12+len(ct))
	out = append(out, byte(len(hdrBytes)>>8), byte(len(hdrBytes)))
	out = append(out, hdrBytes...)
	out = append(out, nonce[:]...)
	out = append(out, ct...)

	return out, nil
}

// RatchetDecrypt decrypts a ciphertext produced by RatchetEncrypt.
// It handles skipped messages by caching their message keys.
func (rs *RatchetState) RatchetDecrypt(ciphertext []byte) ([]byte, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if len(ciphertext) < 2+40+12+16 { // headerLen + header + nonce + minimal GCM tag
		return nil, fmt.Errorf("ratchet decrypt: ciphertext too short (%d bytes)", len(ciphertext))
	}

	// Parse header length
	hdrLen := int(ciphertext[0])<<8 | int(ciphertext[1])
	if hdrLen != 40 {
		return nil, fmt.Errorf("ratchet decrypt: unexpected header length %d", hdrLen)
	}

	// Parse header
	var hdr ratchetHeader
	copy(hdr.DHPub[:], ciphertext[2:34])
	hdr.SendNum = uint32(ciphertext[34])<<24 | uint32(ciphertext[35])<<16 | uint32(ciphertext[36])<<8 | uint32(ciphertext[37])
	hdr.PrevNum = uint32(ciphertext[38])<<24 | uint32(ciphertext[39])<<16 | uint32(ciphertext[40])<<8 | uint32(ciphertext[41])

	rest := ciphertext[2+hdrLen:]
	if len(rest) < 12 {
		return nil, fmt.Errorf("ratchet decrypt: missing nonce")
	}
	var nonce [12]byte
	copy(nonce[:], rest[:12])
	ct := rest[12:]

	// Try skipped keys first
	skKey := skippedKeyEntry{dhPub: hdr.DHPub, msgNum: hdr.SendNum}
	if mk, ok := rs.SkippedKeys[skKey]; ok {
		plain, err := Decrypt(ct, nonce, mk)
		if err == nil {
			delete(rs.SkippedKeys, skKey)
			return plain, nil
		}
		// If decryption fails, fall through to normal path
		delete(rs.SkippedKeys, skKey)
	}

	// Check if this is a new DH ratchet step (remote pub changed)
	if hdr.DHPub != rs.RemotePub {
		// Skip any messages in the current receiving chain
		rs.skipMessages(hdr.PrevNum)

		// Perform DH ratchet step
		if err := rs.dhRatchetStep(hdr.DHPub); err != nil {
			return nil, err
		}
	}

	// Skip messages in the new receiving chain up to hdr.SendNum
	rs.skipMessages(hdr.SendNum)

	// Derive message key from receiving chain
	msgKey := advanceChainKey(&rs.RecvChainKey)
	rs.RecvNum++

	// Decrypt
	var mk [32]byte
	copy(mk[:], msgKey[:])
	return Decrypt(ct, nonce, mk)
}

// skipMessages caches message keys for skipped message numbers in the current
// receiving chain, up to the target number.
func (rs *RatchetState) skipMessages(targetNum uint32) {
	for rs.RecvNum < targetNum {
		// Cache the message key
		msgKey := advanceChainKey(&rs.RecvChainKey)
		key := skippedKeyEntry{dhPub: rs.RemotePub, msgNum: rs.RecvNum}
		rs.SkippedKeys[key] = msgKey
		rs.RecvNum++

		// Evict oldest if over limit
		if len(rs.SkippedKeys) > MaxSkippedKeys {
			rs.evictOldestSkipped()
		}
	}
}

// dhRatchetStep performs a DH ratchet step: computes new shared secret,
// derives new root key and chain keys.
func (rs *RatchetState) dhRatchetStep(remotePub [32]byte) error {
	rs.RemotePub = remotePub

	// DH output
	dhOut, err := ComputeSharedSecret(rs.DHPriv, rs.RemotePub)
	if err != nil {
		return fmt.Errorf("ratchet DH step: %w", err)
	}

	// Root key = HMAC-SHA256(old_root_key, dh_output)
	rs.RootKey = hmacSHA256(rs.RootKey[:], dhOut[:])

	// Derive new receiving chain key
	rs.RecvChainKey = deriveRootChain(rs.RootKey, []byte("RatchetChain"))

	// Generate new DH key pair
	priv, pub, err := GenerateDHKeyPair()
	if err != nil {
		return fmt.Errorf("ratchet generate DH: %w", err)
	}
	rs.DHPriv = priv
	rs.DHPub = pub

	// Second DH step with new keys
	dhOut2, err := ComputeSharedSecret(rs.DHPriv, rs.RemotePub)
	if err != nil {
		return fmt.Errorf("ratchet DH step 2: %w", err)
	}

	// New root key and sending chain
	rs.RootKey = hmacSHA256(rs.RootKey[:], dhOut2[:])
	rs.SendChainKey = deriveRootChain(rs.RootKey, []byte("RatchetSend"))
	rs.SendNum = 0
	rs.RecvNum = 0

	return nil
}

// evictOldestSkipped removes one arbitrary entry from the skipped keys map.
// In a production implementation this would remove the oldest by some metric.
func (rs *RatchetState) evictOldestSkipped() {
	for k := range rs.SkippedKeys {
		delete(rs.SkippedKeys, k)
		return
	}
}

// --- Helper functions ---

// advanceChainKey derives a message key from the chain key and advances the chain.
// chainKey_{n+1} = HMAC-SHA256(chainKey_n, 0x01)
// msgKey_{n}    = HMAC-SHA256(chainKey_n, 0x00)
func advanceChainKey(chainKey *[32]byte) [32]byte {
	msgKey := hmacSHA256(chainKey[:], []byte{0x00})
	newChain := hmacSHA256(chainKey[:], []byte{0x01})
	*chainKey = newChain
	return msgKey
}

// deriveRootChain derives a chain key from a root key and info.
func deriveRootChain(rootKey [32]byte, info []byte) [32]byte {
	return hmacSHA256(rootKey[:], info)
}

// hmacSHA256 computes HMAC-SHA256.
func hmacSHA256(key, data []byte) [32]byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// putUint32 writes a big-endian uint32 into buf (must be at least 4 bytes).
func putUint32(buf []byte, v uint32) {
	buf[0] = byte(v >> 24)
	buf[1] = byte(v >> 16)
	buf[2] = byte(v >> 8)
	buf[3] = byte(v)
}
