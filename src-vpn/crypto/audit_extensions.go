// AUDIT: Crypto extensions — implements remaining crypto tasks.
// T35: AAD, T36: Nonce, T41: Control Flow Obfuscation helper, T42: String encryption,
// T48: R8 rules placeholder, T25: Identity derivation.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"golang.org/x/crypto/hkdf"
)

// T35: Form AAD (Additional Authenticated Data) for AES-GCM
func FormAAD(messageType byte, senderID, recipientID string) []byte {
	h := sha256.New()
	h.Write([]byte{messageType})
	h.Write([]byte(senderID))
	h.Write([]byte(recipientID))
	return h.Sum(nil)
}

// T36: Generate cryptographically secure nonce
func GenerateNonce(size int) ([]byte, error) {
	nonce := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

// T25: Derive Identity ID from Ed25519 public key
func DeriveIdentityID(pubKey []byte) string {
	h := sha256.Sum256(pubKey)
	return hex.EncodeToString(h[:16]) // 16 bytes = 32 hex chars
}

// T42: Encrypt string literal at rest (obfuscation)
func EncryptString(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(sealed), nil
}

// T42: Decrypt string literal
func DecryptString(ciphertextHex string, key []byte) (string, error) {
	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// T35: HKDF-based key derivation for session keys
func DeriveSessionKey(sharedSecret, salt []byte, info string) ([]byte, error) {
	key := make([]byte, 32) // AES-256
	hkdfObj := hkdf.New(sha256.New, sharedSecret, salt, []byte(info))
	if _, err := io.ReadFull(hkdfObj, key); err != nil {
		return nil, err
	}
	return key, nil
}

// T41: Control Flow Flattening dispatcher (obfuscation helper)
// Returns a function that dispatches to one of several handlers based on a state machine.
type ObfuscatedHandler func() int // returns next state

func ControlFlowFlatten(handlers []ObfuscatedHandler) func() {
	return func() {
		state := 0
		for state >= 0 && state < len(handlers) {
			state = handlers[state]()
		}
	}
}
