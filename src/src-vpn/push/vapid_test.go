package push

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

// ==================== VAPID Key Generation Tests ====================

func TestGenerateVAPIDKeys(t *testing.T) {
	priv, pub, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateVAPIDKeys failed: %v", err)
	}

	if priv == "" {
		t.Error("private key is empty")
	}
	if pub == "" {
		t.Error("public key is empty")
	}

	// Private key should be valid base64url
	privBytes, err := base64.RawURLEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("private key not valid base64url: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("private key should be 32 bytes, got %d", len(privBytes))
	}

	// Public key should be valid base64url (uncompressed P-256 = 65 bytes)
	pubBytes, err := base64.RawURLEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("public key not valid base64url: %v", err)
	}
	if len(pubBytes) != 65 {
		t.Errorf("public key should be 65 bytes (uncompressed P-256), got %d", len(pubBytes))
	}
	if pubBytes[0] != 0x04 {
		t.Errorf("public key should start with 0x04 (uncompressed), got 0x%02x", pubBytes[0])
	}
}

func TestGenerateVAPIDKeysUnique(t *testing.T) {
	priv1, pub1, _ := GenerateVAPIDKeys()
	priv2, pub2, _ := GenerateVAPIDKeys()

	if priv1 == priv2 {
		t.Error("two generated private keys should differ")
	}
	if pub1 == pub2 {
		t.Error("two generated public keys should differ")
	}
}

func TestParseVAPIDPrivateKey(t *testing.T) {
	priv, _, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateVAPIDKeys failed: %v", err)
	}

	key, err := parseVAPIDPrivateKey(priv)
	if err != nil {
		t.Fatalf("parseVAPIDPrivateKey failed: %v", err)
	}

	// Verify it's a valid P-256 private key
	if key.Curve != elliptic.P256() {
		t.Error("parsed key curve is not P-256")
	}
	if key.D == nil {
		t.Error("parsed key D is nil")
	}
	if key.X == nil || key.Y == nil {
		t.Error("parsed key public point is nil")
	}

	// Verify the public key is valid
	if !key.Curve.IsOnCurve(key.X, key.Y) {
		t.Error("parsed key public point is not on curve")
	}
}

func TestParseVAPIDPrivateKeyRoundTrip(t *testing.T) {
	priv, pub, _ := GenerateVAPIDKeys()

	key, err := parseVAPIDPrivateKey(priv)
	if err != nil {
		t.Fatalf("parseVAPIDPrivateKey failed: %v", err)
	}

	// Reconstruct the public key bytes and compare
	reconstructedPub := base64urlEncode(elliptic.Marshal(elliptic.P256(), key.X, key.Y))
	if reconstructedPub != pub {
		t.Errorf("public key round-trip mismatch:\nexpected: %s\n got: %s", pub, reconstructedPub)
	}
}

func TestParseVAPIDPrivateKeyInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"not base64", "!!!invalid!!!"},
		{"wrong length", base64urlEncode([]byte{1, 2, 3})},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseVAPIDPrivateKey(tt.input)
			if err == nil {
				t.Error("expected error for invalid private key, got nil")
			}
		})
	}
}

// ==================== VAPID JWT Tests ====================

func TestVapidJWT(t *testing.T) {
	priv, _, _ := GenerateVAPIDKeys()
	key, _ := parseVAPIDPrivateKey(priv)

	jwt, err := vapidJWT(key, "https://fcm.googleapis.com")
	if err != nil {
		t.Fatalf("vapidJWT failed: %v", err)
	}

	if jwt == "" {
		t.Error("JWT is empty")
	}

	// JWT should have 3 parts separated by dots
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Errorf("JWT should have 3 parts, got %d", len(parts))
	}
}

func TestExtractAudience(t *testing.T) {
	tests := []struct {
		endpoint string
		expected string
	}{
		{
			"https://fcm.googleapis.com/fcm/send/abc123",
			"https://fcm.googleapis.com",
		},
		{
			"https://updates.push.services.mozilla.com/wpush/v2/xyz",
			"https://updates.push.services.mozilla.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint[:30], func(t *testing.T) {
			result := extractAudience(tt.endpoint)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestBuildVAPIDHeaders(t *testing.T) {
	priv, pub, _ := GenerateVAPIDKeys()
	key, _ := parseVAPIDPrivateKey(priv)

	cryptoKey, authorization, err := buildVAPIDHeaders(key, pub, "https://fcm.googleapis.com")
	if err != nil {
		t.Fatalf("buildVAPIDHeaders failed: %v", err)
	}

	if cryptoKey == "" {
		t.Error("crypto-key header is empty")
	}
	if authorization == "" {
		t.Error("authorization header is empty")
	}

	// Crypto-Key should contain p256ecdsa=
	if !strings.Contains(cryptoKey, "p256ecdsa=") {
		t.Errorf("crypto-key should contain 'p256ecdsa=', got %q", cryptoKey)
	}

	// Authorization should contain "vapid t="
	if !strings.Contains(authorization, "vapid t=") {
		t.Errorf("authorization should contain 'vapid t=', got %q", authorization)
	}
}

func TestEncryptPushPayload(t *testing.T) {
	// Generate a subscriber key pair for testing
	curve := elliptic.P256()
	privKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("generate subscriber key: %v", err)
	}
	pubBytes := elliptic.Marshal(curve, privKey.X, privKey.Y)
	p256dh := base64urlEncode(pubBytes)

	// Generate auth secret (16 bytes)
	authBytes := make([]byte, 16)
	for i := range authBytes {
		authBytes[i] = byte(i + 1)
	}
	auth := base64urlEncode(authBytes)

	// Encrypt a test payload
	encrypted, err := encryptPushPayload(p256dh, auth, []byte("Hello, Push!"))
	if err != nil {
		t.Fatalf("encryptPushPayload failed: %v", err)
	}

	// Encrypted payload should be longer than plaintext due to overhead
	// salt(16) + rs(4) + pubkey(65) + ciphertext+tag = at least 85 + overhead
	if len(encrypted) < 85 {
		t.Errorf("encrypted payload too short: %d bytes", len(encrypted))
	}
}

// ==================== Base64url Helpers Tests ====================

func TestBase64urlRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"1 byte", []byte{0}},
		{"16 bytes", make([]byte, 16)},
		{"32 bytes", make([]byte, 32)},
		{"65 bytes", make([]byte, 65)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := base64urlEncode(tt.input)
			decoded, err := base64urlDecode(encoded)
			if err != nil {
				t.Fatalf("base64urlDecode failed: %v", err)
			}
			if len(decoded) != len(tt.input) {
				t.Errorf("round-trip length mismatch: expected %d, got %d", len(tt.input), len(decoded))
			}
			for i := range tt.input {
				if decoded[i] != tt.input[i] {
					t.Errorf("round-trip data mismatch at byte %d", i)
					break
				}
			}
		})
	}
}
