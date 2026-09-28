package identity

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil/bech32"
)

// identityJSON is the on-disk format for storing an identity.
type identityJSON struct {
	Nsec string `json:"nsec"`
	Npub string `json:"npub"`
	Hex  string `json:"hex"`
}

// GenerateKeyPair creates a new random secp256k1 private/public key pair.
func GenerateKeyPair() (*btcec.PrivateKey, *btcec.PublicKey, error) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("generate private key: %w", err)
	}
	return privKey, privKey.PubKey(), nil
}

// PrivKeyToNsec encodes a private key as a bech32 "nsec" string.
func PrivKeyToNsec(privKey *btcec.PrivateKey) string {
	encoded, _ := bech32.EncodeFromBase256("nsec", privKey.Serialize())
	return encoded
}

// PubKeyToNpub encodes a public key (compressed) as a bech32 "npub" string.
func PubKeyToNpub(pubKey *btcec.PublicKey) string {
	encoded, _ := bech32.EncodeFromBase256("npub", pubKey.SerializeCompressed())
	return encoded
}

// NsecToPrivKey decodes a bech32 "nsec" string back into a private key.
func NsecToPrivKey(nsec string) (*btcec.PrivateKey, error) {
	hrp, data, err := bech32.DecodeToBase256(nsec)
	if err != nil {
		return nil, fmt.Errorf("decode nsec: %w", err)
	}
	if hrp != "nsec" {
		return nil, errors.New("invalid hrp: expected nsec")
	}
	privKey, _ := btcec.PrivKeyFromBytes(data)
	return privKey, nil
}

// NpubToPubKey decodes a bech32 "npub" string back into a public key.
func NpubToPubKey(npub string) (*btcec.PublicKey, error) {
	hrp, data, err := bech32.DecodeToBase256(npub)
	if err != nil {
		return nil, fmt.Errorf("decode npub: %w", err)
	}
	if hrp != "npub" {
		return nil, errors.New("invalid hrp: expected npub")
	}
	pubKey, err := btcec.ParsePubKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return pubKey, nil
}

// GetPublicKeyHex returns the hex-encoded compressed public key derived from a private key.
func GetPublicKeyHex(privKey *btcec.PrivateKey) string {
	return hex.EncodeToString(privKey.PubKey().SerializeCompressed())
}

// Sign signs the given data with the private key and returns a DER-encoded signature.
func Sign(privKey *btcec.PrivateKey, data []byte) []byte {
	sig := ecdsa.Sign(privKey, data)
	return sig.Serialize()
}

// Verify checks that a DER-encoded signature is valid for the given data and public key.
func Verify(pubKey *btcec.PublicKey, data []byte, sig []byte) bool {
	signature, err := ecdsa.ParseDERSignature(sig)
	if err != nil {
		return false
	}
	return signature.Verify(data, pubKey)
}

// SaveIdentity writes the identity (nsec, npub, hex) to a JSON file.
func SaveIdentity(privKey *btcec.PrivateKey, filePath string) error {
	pubKey := privKey.PubKey()
	ident := identityJSON{
		Nsec: PrivKeyToNsec(privKey),
		Npub: PubKeyToNpub(pubKey),
		Hex:  GetPublicKeyHex(privKey),
	}

	data, err := json.MarshalIndent(ident, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal identity: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("write identity file: %w", err)
	}
	return nil
}

// LoadIdentity reads a JSON identity file and returns the private key.
func LoadIdentity(filePath string) (*btcec.PrivateKey, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read identity file: %w", err)
	}

	var ident identityJSON
	if err := json.Unmarshal(data, &ident); err != nil {
		return nil, fmt.Errorf("unmarshal identity: %w", err)
	}

	privKey, err := NsecToPrivKey(ident.Nsec)
	if err != nil {
		return nil, fmt.Errorf("decode nsec from identity: %w", err)
	}
	return privKey, nil
}
