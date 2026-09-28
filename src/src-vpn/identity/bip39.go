package identity

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/tyler-smith/go-bip39"
)

// GenerateMnemonic creates a new 12-word BIP39 mnemonic.
func GenerateMnemonic() (string, error) {
	// 128 bits of entropy = 12 words
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return "", fmt.Errorf("generate entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("generate mnemonic: %w", err)
	}
	return mnemonic, nil
}

// MnemonicToSeed converts a BIP39 mnemonic to a 64-byte seed (empty passphrase).
func MnemonicToSeed(mnemonic string) []byte {
	return MnemonicToSeedWithPhrase(mnemonic, "")
}

// MnemonicToSeedWithPhrase is table #59: 12 BIP39 words + optional own phrase.
// The phrase is the BIP39 passphrase ("25th word"), not a brainwallet.
func MnemonicToSeedWithPhrase(mnemonic, phrase string) []byte {
	return bip39.NewSeed(mnemonic, phrase)
}

// IsMnemonicValid reports whether s is a BIP39 word list mnemonic.
func IsMnemonicValid(s string) bool {
	return bip39.IsMnemonicValid(s)
}

// SeedToPrivKey derives a secp256k1 private key from a BIP39 seed.
// It uses the first 32 bytes of the seed as the private key scalar.
func SeedToPrivKey(seed []byte) *btcec.PrivateKey {
	// Take first 32 bytes as private key material.
	// PrivKeyFromBytes treats the input as a big-endian scalar and
	// reduces it modulo the curve order, so a 32-byte seed slice works.
	privKey, _ := btcec.PrivKeyFromBytes(seed[:32])
	return privKey
}

// PrivKeyToMnemonic converts a private key back to a BIP39 mnemonic.
// This encodes the 32-byte private key as entropy and generates a mnemonic.
// Note: this produces a valid BIP39 mnemonic that maps back to the same key,
// but it will be 24 words (256 bits) because the full 32-byte key is used as entropy.
func PrivKeyToMnemonic(privKey *btcec.PrivateKey) (string, error) {
	mnemonic, err := bip39.NewMnemonic(privKey.Serialize())
	if err != nil {
		return "", fmt.Errorf("encode private key as mnemonic: %w", err)
	}
	return mnemonic, nil
}
