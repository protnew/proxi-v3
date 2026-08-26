package identity

import (
	"bytes"
	"strings"
	"testing"
)

// Table #59 winner: BIP39 12 words + optional own phrase (25th word).
// Brainwallet (phrase = the whole key) is rejected.

func TestMnemonicToSeedWithPhrase_EmptyEqualsOld(t *testing.T) {
	m, err := GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	old := MnemonicToSeed(m)
	got := MnemonicToSeedWithPhrase(m, "")
	if !bytes.Equal(old, got) {
		t.Fatal("empty phrase must match MnemonicToSeed")
	}
}

func TestMnemonicToSeedWithPhrase_SamePhraseSameSeed(t *testing.T) {
	m, err := GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	a := MnemonicToSeedWithPhrase(m, "моя осмысленная фраза")
	b := MnemonicToSeedWithPhrase(m, "моя осмысленная фраза")
	if !bytes.Equal(a, b) {
		t.Fatal("same mnemonic+phrase must be deterministic")
	}
	if len(a) != 64 {
		t.Fatalf("seed len %d, want 64", len(a))
	}
}

func TestMnemonicToSeedWithPhrase_DifferentPhraseDifferentKey(t *testing.T) {
	m, err := GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	a := SeedToPrivKey(MnemonicToSeedWithPhrase(m, "первая фраза"))
	b := SeedToPrivKey(MnemonicToSeedWithPhrase(m, "вторая фраза"))
	if bytes.Equal(a.Serialize(), b.Serialize()) {
		t.Fatal("different phrases must produce different keys")
	}
}

func TestMnemonicToSeedWithPhrase_NotBrainwallet(t *testing.T) {
	// A Russian sentence alone is NOT a BIP39 mnemonic.
	if IsMnemonicValid("моя осмысленная фраза для входа") {
		t.Fatal("plain sentence must not be a valid mnemonic")
	}
}

func TestMnemonicToSeedWithPhrase_TrimsNothingSecret(t *testing.T) {
	m, err := GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	// Leading/trailing spaces in passphrase MUST change the seed (BIP39 does not trim passphrase).
	a := MnemonicToSeedWithPhrase(m, "secret")
	b := MnemonicToSeedWithPhrase(m, " secret ")
	if bytes.Equal(a, b) {
		t.Fatal("passphrase is exact; spaces are part of the secret")
	}
	_ = strings.TrimSpace
}
