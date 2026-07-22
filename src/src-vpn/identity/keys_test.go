package identity

import (
	"strings"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if priv == nil {
		t.Error("private key should not be nil")
	}
	if pub == nil {
		t.Error("public key should not be nil")
	}

	npub := PubKeyToNpub(pub)
	nsec := PrivKeyToNsec(priv)

	if !strings.HasPrefix(npub, "npub1") {
		t.Errorf("npub should start with 'npub1', got %q", npub)
	}
	if !strings.HasPrefix(nsec, "nsec1") {
		t.Errorf("nsec should start with 'nsec1', got %q", nsec)
	}
	if len(npub) < 20 {
		t.Errorf("npub seems too short: %q", npub)
	}
	if len(nsec) < 20 {
		t.Errorf("nsec seems too short: %q", nsec)
	}
}

func TestPrivKeyToNsecRoundtrip(t *testing.T) {
	priv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	nsec := PrivKeyToNsec(priv)
	recovered, err := NsecToPrivKey(nsec)
	if err != nil {
		t.Fatalf("NsecToPrivKey: %v", err)
	}

	if priv.Serialize() == nil || recovered.Serialize() == nil {
		t.Fatal("serialize returned nil")
	}

	origBytes := priv.Serialize()
	recBytes := recovered.Serialize()
	if len(origBytes) != len(recBytes) {
		t.Fatalf("key length mismatch: %d vs %d", len(origBytes), len(recBytes))
	}
	for i := range origBytes {
		if origBytes[i] != recBytes[i] {
			t.Error("private key roundtrip failed: bytes differ")
			break
		}
	}
}

func TestPubKeyToNpubRoundtrip(t *testing.T) {
	_, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	npub := PubKeyToNpub(pub)
	recovered, err := NpubToPubKey(npub)
	if err != nil {
		t.Fatalf("NpubToPubKey: %v", err)
	}

	origBytes := pub.SerializeCompressed()
	recBytes := recovered.SerializeCompressed()
	if len(origBytes) != len(recBytes) {
		t.Fatalf("key length mismatch: %d vs %d", len(origBytes), len(recBytes))
	}
	for i := range origBytes {
		if origBytes[i] != recBytes[i] {
			t.Error("public key roundtrip failed: bytes differ")
			break
		}
	}
}
