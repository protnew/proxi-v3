// Package push implements Web Push notifications with VAPID authentication.
//
// vapid.go implements VAPID (Voluntary Application Server Identification)
// per RFC 8291 + RFC 8292 + Web Push RFC 8030.
//
// VAPID allows the application server to identify itself to the push service
// using ECDSA P-256 signatures, eliminating the need for per-subscription
// authentication tokens.
package push

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// ==================== Base64url helpers ====================

func base64urlEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64urlDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// ==================== VAPID Key Generation ====================

// GenerateVAPIDKeys generates a new ECDSA P-256 key pair for VAPID.
// Returns base64url-encoded (unpadded) private key and public key.
func GenerateVAPIDKeys() (privateKey, publicKey string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ECDSA P-256 key: %w", err)
	}

	// Private key: raw 32-byte scalar, base64url-unpadded
	privBytes := key.D.Bytes()
	// Ensure 32 bytes (leading zero padding)
	if len(privBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(privBytes):], privBytes)
		privBytes = padded
	}
	privateKey = base64urlEncode(privBytes)

	// Public key: uncompressed point (65 bytes: 0x04 + X + Y), base64url-unpadded
	pubBytes := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)
	publicKey = base64urlEncode(pubBytes)

	return privateKey, publicKey, nil
}

// ==================== VAPID JWT ====================

// vapidJWT creates a VAPID JWT token signed with ECDSA P-256.
func vapidJWT(vapidPrivateKey *ecdsa.PrivateKey, audience string) (string, error) {
	// JWT header
	header := map[string]interface{}{
		"typ": "JWT",
		"alg": "ES256",
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64urlEncode(headerJSON)

	// JWT payload
	now := time.Now()
	payload := map[string]interface{}{
		"aud": audience,
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": "mailto:admin@unkillable-messenger.local",
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64urlEncode(payloadJSON)

	// Sign
	signingInput := headerB64 + "." + payloadB64
	hash := sha256.Sum256([]byte(signingInput))
	sig, err := vapidPrivateKey.Sign(rand.Reader, hash[:], crypto.SHA256)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}

	signatureB64 := base64urlEncode(sig)
	return signingInput + "." + signatureB64, nil
}

// extractAudience extracts the origin (scheme://host) from a push endpoint URL.
func extractAudience(endpoint string) string {
	parts := strings.SplitN(endpoint, "/", 4)
	if len(parts) >= 3 {
		return parts[0] + "//" + parts[2]
	}
	return endpoint
}

// parseVAPIDPrivateKey decodes a base64url-encoded private key into an ECDSA private key.
func parseVAPIDPrivateKey(privateKeyB64 string) (*ecdsa.PrivateKey, error) {
	privBytes, err := base64urlDecode(privateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(privBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: %d bytes, expected 32", len(privBytes))
	}

	curve := elliptic.P256()
	x, y := curve.ScalarBaseMult(privBytes)
	if x == nil {
		return nil, fmt.Errorf("invalid private key scalar")
	}

	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: new(big.Int).SetBytes(privBytes),
	}, nil
}

// buildVAPIDHeaders constructs the Authorization header for VAPID.
func buildVAPIDHeaders(vapidPrivateKey *ecdsa.PrivateKey, vapidPublicKey string, audience string) (cryptoKey, authorization string, err error) {
	jwt, err := vapidJWT(vapidPrivateKey, audience)
	if err != nil {
		return "", "", fmt.Errorf("build VAPID JWT: %w", err)
	}

	// Crypto-Key header includes the VAPID public key
	cryptoKey = "p256ecdsa=" + vapidPublicKey

	// Authorization header with WebPush vapid scheme
	authorization = "vapid t=" + jwt + ", k=" + vapidPublicKey

	return cryptoKey, authorization, nil
}
