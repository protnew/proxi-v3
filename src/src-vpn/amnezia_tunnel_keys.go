package vpn

// amnezia_tunnel_keys.go — X25519 key helpers for WG conf (uses crypto package).

import (
	"encoding/base64"
	"fmt"

	"github.com/unkillable-messenger/vpn/crypto"
)

// GenerateDHKeyPairFromVPN returns X25519 pair for WireGuard-compatible keys.
func GenerateDHKeyPairFromVPN() (priv, pub [32]byte, err error) {
	return crypto.GenerateDHKeyPair()
}

// PublicKeyFromPrivateB64 derives X25519 public key from base64 private key.
func PublicKeyFromPrivateB64(privB64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		// try raw url encoding
		raw, err = base64.RawStdEncoding.DecodeString(privB64)
		if err != nil {
			return "", fmt.Errorf("decode private key: %w", err)
		}
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("private key must be 32 bytes, got %d", len(raw))
	}
	var priv [32]byte
	copy(priv[:], raw)
	// public = X25519(priv, basepoint) via ComputeSharedSecret pattern in crypto
	// Use Generate path: crypto has no PublicFromPrivate — compute via curve through DH with basepoint
	pub, err := crypto.PublicFromPrivate(priv)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(pub[:]), nil
}
