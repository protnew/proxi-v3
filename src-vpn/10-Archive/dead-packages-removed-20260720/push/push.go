// Package push implements Web Push notifications with VAPID authentication.
//
// push.go implements push subscription storage and notification sending.
// Subscriptions are persisted in SQLite. Push messages are sent via HTTP POST
// to the push service endpoint using VAPID authentication (RFC 8291/8292).
package push

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ==================== Push Subscription Types ====================

// PushSubscription represents a Web Push subscription stored in SQLite.
type PushSubscription struct {
	ID        int64  `json:"id"`
	UserID    string `json:"userId"`
	Endpoint  string `json:"endpoint"`
	P256DH    string `json:"p256dh"` // subscriber's ECDH P-256 public key (base64url)
	Auth      string `json:"auth"`   // subscriber's auth secret (16 bytes, base64url)
	CreatedAt int64  `json:"createdAt"`
}

// ==================== Push Encryption (RFC 8291) ====================

// encryptPushPayload encrypts the payload using ECDH + AES-128-GCM
// per RFC 8291 (Message Encryption for Web Push).
func encryptPushPayload(p256dh, auth string, payload []byte) ([]byte, error) {
	// Decode subscriber's public key and auth secret
	subscriberPubBytes, err := base64urlDecode(p256dh)
	if err != nil {
		return nil, fmt.Errorf("decode p256dh: %w", err)
	}
	authSecret, err := base64urlDecode(auth)
	if err != nil {
		return nil, fmt.Errorf("decode auth: %w", err)
	}
	if len(authSecret) != 16 {
		return nil, fmt.Errorf("auth secret must be 16 bytes, got %d", len(authSecret))
	}

	// Generate ephemeral ECDH P-256 key pair
	curve := ecdh.P256()
	ephemeralPriv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}

	// Parse subscriber's public key (raw 65-byte uncompressed point for P-256)
	subscriberPub, err := curve.NewPublicKey(subscriberPubBytes)
	if err != nil {
		return nil, fmt.Errorf("parse subscriber public key: %w", err)
	}

	// ECDH shared secret
	sharedSecret, err := ephemeralPriv.ECDH(subscriberPub)
	if err != nil {
		return nil, fmt.Errorf("ECDH: %w", err)
	}

	// RFC 8291: IKM = HMAC-SHA-256(auth_secret, shared_secret)
	ikm := hmacSHA256(authSecret, sharedSecret)

	// RFC 8291: PRK = HMAC-SHA-256(salt, IKM)
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	prk := hmacSHA256(salt, ikm)

	// RFC 8291: CEK and Nonce derivation via HKDF
	cekInfo := buildInfo("Content-Encoding: aes128gcm\000", subscriberPubBytes)
	cek := hmacSHA256SingleByte(prk, cekInfo, 0x01)

	nonceInfo := buildInfo("Content-Encoding: nonce\000", subscriberPubBytes)
	nonceKey := hmacSHA256SingleByte(prk, nonceInfo, 0x01)
	nonce := nonceKey[:12] // AES-GCM uses 12-byte nonce

	// Encrypt with AES-128-GCM
	block, err := aes.NewCipher(cek[:16])
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// RFC 8291: add padding delimiter (0x02 for no padding)
	paddedPayload := append(payload, 0x02)
	ciphertext := aead.Seal(nil, nonce, paddedPayload, nil)

	// RFC 8291 wire format: salt(16) + rs(4) + pubkey(65) + ciphertext+tag
	rs := uint32(4096) // record size
	result := make([]byte, 0, 16+4+len(ephemeralPriv.PublicKey().Bytes())+len(ciphertext))
	result = append(result, salt...)
	rsBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(rsBuf, rs)
	result = append(result, rsBuf...)
	result = append(result, ephemeralPriv.PublicKey().Bytes()...)
	result = append(result, ciphertext...)

	return result, nil
}

// buildInfo constructs the HKDF info parameter for RFC 8291.
func buildInfo(encoding string, pubKey []byte) []byte {
	info := []byte(encoding)
	pubKeyLen := make([]byte, 2)
	binary.BigEndian.PutUint16(pubKeyLen, uint16(len(pubKey)))
	info = append(info, pubKeyLen...)
	info = append(info, pubKey...)
	return info
}

// hmacSHA256 computes HMAC-SHA-256.
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// hmacSHA256SingleByte computes HMAC-SHA-256 with a single byte appended.
func hmacSHA256SingleByte(key, data []byte, b byte) []byte {
	combined := make([]byte, len(data)+1)
	copy(combined, data)
	combined[len(data)] = b
	return hmacSHA256(key, combined)
}

// ==================== Send Push Notification ====================

// SendPushNotification sends an encrypted push notification to a subscriber.
// It encrypts the payload with RFC 8291 and sends via HTTP POST with VAPID auth.
func SendPushNotification(sub PushSubscription, payload []byte, vapidPrivateKey, vapidPublicKey string) error {
	// 1. Encrypt the payload per RFC 8291
	encrypted, err := encryptPushPayload(sub.P256DH, sub.Auth, payload)
	if err != nil {
		return fmt.Errorf("encrypt push payload: %w", err)
	}

	// 2. Parse VAPID private key
	vapidPriv, err := parseVAPIDPrivateKey(vapidPrivateKey)
	if err != nil {
		return fmt.Errorf("parse VAPID key: %w", err)
	}

	// 3. Build VAPID headers
	audience := extractAudience(sub.Endpoint)
	cryptoKey, authorization, err := buildVAPIDHeaders(vapidPriv, vapidPublicKey, audience)
	if err != nil {
		return fmt.Errorf("build VAPID headers: %w", err)
	}

	// 4. Send HTTP POST to push service
	req, err := http.NewRequest("POST", sub.Endpoint, bytes.NewReader(encrypted))
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", "86400") // 24 hours
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Crypto-Key", cryptoKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send push request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("push service returned %d: %s", resp.StatusCode, resp.Status)
	}

	log.Printf("🔔 Push sent to %s (status %d)", sub.UserID, resp.StatusCode)
	return nil
}

// ==================== Build Push Message ====================

// BuildPushMessage encrypts and builds the complete push message body.
// Useful for debugging or custom transport.
func BuildPushMessage(endpoint, vapidPrivateKey, vapidPublicKey, subscriberAuth string, payload []byte) ([]byte, error) {
	// This is a convenience wrapper that just returns the encrypted payload
	encrypted, err := encryptPushPayload(vapidPublicKey, subscriberAuth, payload)
	if err != nil {
		return nil, fmt.Errorf("build push message: %w", err)
	}
	return encrypted, nil
}
