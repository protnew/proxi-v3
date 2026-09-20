// Package nostr implements the Nostr protocol.
//
// nip44.go implements NIP-44 v2 encryption.
// Spec: https://github.com/nostr-protocol/nips/blob/master/44.md
//
// P6 (2026-09-20): переписано строго по спеке — предыдущая версия была
// wire-несовместима с nostr-tools (PWA) и официальными векторами:
// использовался XChaCha20-Poly1305 с 24-байтным nonce и PKCS#7 вместо
// HKDF-Expand message keys + ChaCha20 + HMAC-SHA256 + length-prefix padding.
//
//	conversation_key = HKDF-Extract(salt="nip44-v2", ikm=ecdh_shared_x)
//	message_keys     = HKDF-Expand(conversation_key, info=nonce, L=76)
//	ciphertext       = ChaCha20(chacha_key, chacha_nonce, padded)
//	mac              = HMAC-SHA256(hmac_key, nonce || ciphertext)
//	wire             = base64(version[0x02] + nonce[32] + ciphertext + mac[32])
package nostr

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"unicode/utf8"

	"github.com/btcsuite/btcd/btcec/v2"
	"golang.org/x/crypto/chacha20"
	"golang.org/x/crypto/hkdf"
)

// NIP-44 version byte
const nip44Version = 0x02

// Spec limits
const (
	nip44MinPlaintext  = 1
	nip44MaxPlaintext  = 65535
	nip44NonceLen      = 32
	nip44MACLen        = 32
	nip44MinPadded     = 32
	nip44MinPayloadLen = 1 + nip44NonceLen + nip44MinPadded + nip44MACLen // 99
)

// ==================== Conversation Key ====================

// computeConversationKey derives the NIP-44 v2 conversation key from ECDH.
// conversation_key = HKDF-Extract(salt="nip44-v2", ikm=shared_x)
func computeConversationKey(privKey *btcec.PrivateKey, pubKey *btcec.PublicKey) ([32]byte, error) {
	sharedX := btcec.GenerateSharedSecret(privKey, pubKey)
	mac := hmac.New(sha256.New, []byte("nip44-v2"))
	mac.Write(sharedX)

	var key [32]byte
	copy(key[:], mac.Sum(nil))
	return key, nil
}

// ==================== Message Keys ====================

// nip44MessageKeys expands per-message keys from conversation key + nonce.
// HKDF-Expand(prk=conversation_key, info=nonce, L=76) → key|nonce|hmac.
func nip44MessageKeys(convKey [32]byte, nonce []byte) (chachaKey, chachaNonce, hmacKey []byte, err error) {
	if len(nonce) != nip44NonceLen {
		return nil, nil, nil, fmt.Errorf("nip44: nonce must be %d bytes", nip44NonceLen)
	}
	r := hkdf.Expand(sha256.New, convKey[:], nonce)
	keys := make([]byte, 76)
	if _, err := io.ReadFull(r, keys); err != nil {
		return nil, nil, nil, fmt.Errorf("nip44: hkdf expand: %w", err)
	}
	return keys[0:32], keys[32:44], keys[44:76], nil
}

// ==================== Padding ====================

// calcPaddedLen returns the NIP-44 v2 bucket size for a plaintext length.
func calcPaddedLen(unpadded int) int {
	if unpadded <= nip44MinPadded {
		return nip44MinPadded
	}
	nextPower := 1 << bits.Len(uint(unpadded-1))
	chunk := nextPower / 8
	if chunk < 32 {
		chunk = 32
	}
	return chunk * ((unpadded-1)/chunk + 1)
}

// nip44Pad applies NIP-44 v2 padding: u16-BE length + data + zero fill.
func nip44Pad(plaintext []byte) ([]byte, error) {
	plen := len(plaintext)
	if plen < nip44MinPlaintext || plen > nip44MaxPlaintext {
		return nil, fmt.Errorf("nip44: plaintext length %d out of range [%d,%d]", plen, nip44MinPlaintext, nip44MaxPlaintext)
	}
	if !utf8.Valid(plaintext) {
		return nil, fmt.Errorf("nip44: plaintext is not valid UTF-8")
	}
	paddedLen := calcPaddedLen(plen)
	out := make([]byte, 2+paddedLen)
	binary.BigEndian.PutUint16(out[:2], uint16(plen))
	copy(out[2:], plaintext)
	return out, nil
}

// nip44Unpad validates and strips NIP-44 v2 padding.
func nip44Unpad(data []byte) ([]byte, error) {
	if len(data) < 2+nip44MinPadded {
		return nil, fmt.Errorf("nip44: padded data too short")
	}
	ulen := int(binary.BigEndian.Uint16(data[:2]))
	if ulen < nip44MinPlaintext || ulen > nip44MaxPlaintext {
		return nil, fmt.Errorf("nip44: invalid unpadded length %d", ulen)
	}
	if len(data) != 2+calcPaddedLen(ulen) {
		return nil, fmt.Errorf("nip44: padded length %d does not match bucket for %d", len(data), ulen)
	}
	payload := data[2 : 2+ulen]
	for _, b := range data[2+ulen:] {
		if b != 0 {
			return nil, fmt.Errorf("nip44: non-zero padding byte")
		}
	}
	if !utf8.Valid(payload) {
		return nil, fmt.Errorf("nip44: decrypted plaintext is not valid UTF-8")
	}
	return payload, nil
}

// ==================== Encrypt / Decrypt ====================

// Encrypt44 encrypts plaintext using NIP-44 v2.
// Returns base64(version[0x02] + nonce[32] + ciphertext + mac[32]).
func Encrypt44(senderPrivKey *btcec.PrivateKey, recipientPubKey *btcec.PublicKey, plaintext string) (string, error) {
	nonce := make([]byte, nip44NonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nip44 encrypt: generate nonce: %w", err)
	}
	return encrypt44WithNonce(senderPrivKey, recipientPubKey, plaintext, nonce)
}

// encrypt44WithNonce is the deterministic core (used by tests against
// official vectors; nonce MUST be fresh CSPRNG in production paths).
func encrypt44WithNonce(senderPrivKey *btcec.PrivateKey, recipientPubKey *btcec.PublicKey, plaintext string, nonce []byte) (string, error) {
	convKey, err := computeConversationKey(senderPrivKey, recipientPubKey)
	if err != nil {
		return "", fmt.Errorf("nip44 encrypt: conversation key: %w", err)
	}

	chachaKey, chachaNonce, hmacKey, err := nip44MessageKeys(convKey, nonce)
	if err != nil {
		return "", fmt.Errorf("nip44 encrypt: %w", err)
	}

	padded, err := nip44Pad([]byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("nip44 encrypt: %w", err)
	}

	ciphertext := make([]byte, len(padded))
	c, err := chacha20.NewUnauthenticatedCipher(chachaKey, chachaNonce)
	if err != nil {
		return "", fmt.Errorf("nip44 encrypt: chacha20: %w", err)
	}
	c.XORKeyStream(ciphertext, padded)

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(nonce)
	mac.Write(ciphertext)

	payload := make([]byte, 0, 1+nip44NonceLen+len(ciphertext)+nip44MACLen)
	payload = append(payload, nip44Version)
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)
	payload = append(payload, mac.Sum(nil)...)

	return base64.StdEncoding.EncodeToString(payload), nil
}

// Decrypt44 decrypts a NIP-44 v2 payload (base64).
func Decrypt44(recipientPrivKey *btcec.PrivateKey, senderPubKey *btcec.PublicKey, ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: base64 decode: %w", err)
	}
	if len(data) < nip44MinPayloadLen || len(data) > 1+nip44NonceLen+2+nip44MaxPlaintext+nip44MACLen {
		return "", fmt.Errorf("nip44 decrypt: invalid payload length %d", len(data))
	}
	if data[0] != nip44Version {
		return "", fmt.Errorf("nip44 decrypt: unsupported version %d, expected %d", data[0], nip44Version)
	}

	nonce := data[1 : 1+nip44NonceLen]
	macBytes := data[len(data)-nip44MACLen:]
	enc := data[1+nip44NonceLen : len(data)-nip44MACLen]

	convKey, err := computeConversationKey(recipientPrivKey, senderPubKey)
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: conversation key: %w", err)
	}
	chachaKey, chachaNonce, hmacKey, err := nip44MessageKeys(convKey, nonce)
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: %w", err)
	}

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(nonce)
	mac.Write(enc)
	if !hmac.Equal(macBytes, mac.Sum(nil)) {
		return "", fmt.Errorf("nip44 decrypt: invalid MAC")
	}

	padded := make([]byte, len(enc))
	c, err := chacha20.NewUnauthenticatedCipher(chachaKey, chachaNonce)
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: chacha20: %w", err)
	}
	c.XORKeyStream(padded, enc)

	plaintext, err := nip44Unpad(padded)
	if err != nil {
		return "", fmt.Errorf("nip44 decrypt: %w", err)
	}
	return string(plaintext), nil
}
