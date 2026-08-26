package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// GenerateEd25519 returns pub,priv hex (CRYP-012).
func GenerateEd25519() (pubHex, privHex string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(pub), hex.EncodeToString(priv), nil
}

// SignMessageEd25519 signs canonical payload from|to|text|timestamp.
func SignMessageEd25519(privHex, from, to, text string, ts int64) (string, error) {
	priv, err := hex.DecodeString(privHex)
	if err != nil || len(priv) != ed25519.PrivateKeySize {
		// try seed 32-byte
		if len(priv) == ed25519.SeedSize {
			priv = ed25519.NewKeyFromSeed(priv)
		} else {
			return "", fmt.Errorf("invalid ed25519 private key")
		}
	}
	payload := []byte(fmt.Sprintf("%s|%s|%s|%d", from, to, text, ts))
	sig := ed25519.Sign(ed25519.PrivateKey(priv), payload)
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifyMessageEd25519 verifies SignMessageEd25519 output.
func VerifyMessageEd25519(pubHex, from, to, text string, ts int64, sigB64 string) bool {
	pub, err := hex.DecodeString(pubHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return false
	}
	payload := []byte(fmt.Sprintf("%s|%s|%s|%d", from, to, text, ts))
	return ed25519.Verify(ed25519.PublicKey(pub), payload, sig)
}
