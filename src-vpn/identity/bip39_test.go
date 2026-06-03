package identity

import (
	"strings"
	"testing"

	"github.com/tyler-smith/go-bip39"
)

func TestGenerateMnemonic(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("GenerateMnemonic: %v", err)
	}
	if mnemonic == "" {
		t.Fatal("mnemonic should not be empty")
	}
	words := strings.Fields(mnemonic)
	if len(words) != 12 {
		t.Fatalf("expected 12-word mnemonic, got %d words", len(words))
	}
	// Validate the mnemonic is a valid BIP39 mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		t.Errorf("generated mnemonic is not valid BIP39: %q", mnemonic)
	}
}

func TestGenerateMnemonic_Uniqueness(t *testing.T) {
	m1, err := GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	m2, err := GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	if m1 == m2 {
		t.Error("two generated mnemonics should differ")
	}
}

func TestMnemonicToSeed(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	seed := MnemonicToSeed(mnemonic)
	if len(seed) != 64 {
		t.Fatalf("expected 64-byte seed, got %d bytes", len(seed))
	}
	// Same mnemonic should produce same seed
	seed2 := MnemonicToSeed(mnemonic)
	for i := range seed {
		if seed[i] != seed2[i] {
			t.Error("same mnemonic should produce same seed")
			break
		}
	}
}

func TestMnemonicToSeed_DifferentMnemnonic_DifferentSeed(t *testing.T) {
	m1, _ := GenerateMnemonic()
	m2, _ := GenerateMnemonic()
	s1 := MnemonicToSeed(m1)
	s2 := MnemonicToSeed(m2)
	match := true
	for i := range s1 {
		if s1[i] != s2[i] {
			match = false
			break
		}
	}
	if match {
		t.Error("different mnemonics should produce different seeds")
	}
}

func TestSeedToPrivKey(t *testing.T) {
	seed := MnemonicToSeed("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	privKey := SeedToPrivKey(seed)
	if privKey == nil {
		t.Fatal("SeedToPrivKey returned nil")
	}
	// Should produce a valid public key
	pubKey := privKey.PubKey()
	if pubKey == nil {
		t.Error("PubKey() returned nil")
	}
	// The private key bytes should be deterministic for the same seed
	privKey2 := SeedToPrivKey(seed)
	if privKey.Serialize() == nil || privKey2.Serialize() == nil {
		t.Fatal("Serialize returned nil")
	}
	b1 := privKey.Serialize()
	b2 := privKey2.Serialize()
	for i := range b1 {
		if b1[i] != b2[i] {
			t.Error("same seed should produce same private key")
			break
		}
	}
}

func TestSeedToPrivKey_RoundtripWithMnemonic(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	seed := MnemonicToSeed(mnemonic)
	privKey := SeedToPrivKey(seed)
	if privKey == nil {
		t.Fatal("SeedToPrivKey returned nil")
	}
	// PrivKey should be usable for signing
	data := []byte("test data for signing")
	sig := Sign(privKey, data)
	if len(sig) == 0 {
		t.Error("Sign returned empty signature")
	}
	if !Verify(privKey.PubKey(), data, sig) {
		t.Error("signature verification failed")
	}
}

func TestPrivKeyToMnemonic(t *testing.T) {
	priv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	mnemonic, err := PrivKeyToMnemonic(priv)
	if err != nil {
		t.Fatalf("PrivKeyToMnemonic: %v", err)
	}
	// Should produce a 24-word mnemonic (256-bit entropy = 32-byte private key)
	words := strings.Fields(mnemonic)
	if len(words) != 24 {
		t.Fatalf("expected 24-word mnemonic from private key, got %d", len(words))
	}
	// Validate BIP39
	if !bip39.IsMnemonicValid(mnemonic) {
		t.Errorf("PrivKeyToMnemonic produced invalid BIP39: %q", mnemonic)
	}
}

func TestPrivKeyToMnemonic_ProducesValidMnemonic(t *testing.T) {
	priv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	mnemonic, err := PrivKeyToMnemonic(priv)
	if err != nil {
		t.Fatal(err)
	}
	// The mnemonic should be valid BIP39
	if !bip39.IsMnemonicValid(mnemonic) {
		t.Fatalf("PrivKeyToMnemonic produced invalid mnemonic: %q", mnemonic)
	}

	// The mnemonic's entropy should encode the original private key bytes
	// so we can recover them from the mnemonic's entropy
	entropy, err := bip39.EntropyFromMnemonic(mnemonic)
	if err != nil {
		t.Fatalf("EntropyFromMnemonic: %v", err)
	}
	if len(entropy) != 32 {
		t.Fatalf("expected 32 bytes entropy, got %d", len(entropy))
	}

	// The entropy should match the serialized private key
	origBytes := priv.Serialize()
	for i := range origBytes {
		if entropy[i] != origBytes[i] {
			t.Error("entropy from mnemonic should match private key bytes")
			break
		}
	}

	// Reconstruct private key from the entropy (pad to 32 bytes for SeedToPrivKey)
	padded := append(entropy, make([]byte, 32)...)
	recovered := SeedToPrivKey(padded)
	recBytes := recovered.Serialize()
	for i := range origBytes {
		if recBytes[i] != origBytes[i] {
			t.Error("reconstructed key should match original")
			break
		}
	}
}

func TestMnemonicToSeed_EmptyPassword(t *testing.T) {
	// MnemonicToSeed uses empty password by default
	// Verify that explicit bip39.NewSeed with "" matches
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	seed1 := MnemonicToSeed(mnemonic)
	seed2 := bip39.NewSeed(mnemonic, "")
	for i := range seed1 {
		if seed1[i] != seed2[i] {
			t.Error("MnemonicToSeed should use empty password")
			break
		}
	}
}
