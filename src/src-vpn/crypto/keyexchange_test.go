package crypto

import (
	"crypto/ed25519"
	"testing"
)

func TestGeneratePreKeyBundle(t *testing.T) {
	// Generate Ed25519 identity key
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	bundle, err := GeneratePreKeyBundle(priv)
	if err != nil {
		t.Fatalf("GeneratePreKeyBundle: %v", err)
	}

	if len(bundle.IdentityKey) != ed25519.PublicKeySize {
		t.Errorf("expected identity key size %d, got %d", ed25519.PublicKeySize, len(bundle.IdentityKey))
	}
	if len(bundle.SignedPreKey) != 32 {
		t.Errorf("expected signed prekey size 32, got %d", len(bundle.SignedPreKey))
	}
	if len(bundle.Signature) != ed25519.SignatureSize {
		t.Errorf("expected signature size %d, got %d", ed25519.SignatureSize, len(bundle.Signature))
	}
	if len(bundle.OneTimePreKey) != 32 {
		t.Errorf("expected one-time prekey size 32, got %d", len(bundle.OneTimePreKey))
	}
}

func TestVerifyPreKeyBundle(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	bundle, err := GeneratePreKeyBundle(priv)
	if err != nil {
		t.Fatalf("GeneratePreKeyBundle: %v", err)
	}

	if !VerifyPreKeyBundle(bundle) {
		t.Error("VerifyPreKeyBundle should return true for valid bundle")
	}

	// Tamper with the signed prekey — should fail
	tampered := &PreKeyBundle{
		IdentityKey:   bundle.IdentityKey,
		SignedPreKey:  append([]byte{0xFF}, bundle.SignedPreKey[1:]...),
		Signature:     bundle.Signature,
		OneTimePreKey: bundle.OneTimePreKey,
	}
	if VerifyPreKeyBundle(tampered) {
		t.Error("VerifyPreKeyBundle should return false for tampered bundle")
	}

	// Nil bundle
	if VerifyPreKeyBundle(nil) {
		t.Error("VerifyPreKeyBundle should return false for nil bundle")
	}
}

func TestDeriveSharedSecret(t *testing.T) {
	privA, pubA, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("GenerateDHKeyPair A: %v", err)
	}
	privB, pubB, err := GenerateDHKeyPair()
	if err != nil {
		t.Fatalf("GenerateDHKeyPair B: %v", err)
	}

	sharedAB, err := DeriveSharedSecret(privA[:], pubB[:])
	if err != nil {
		t.Fatalf("DeriveSharedSecret A→B: %v", err)
	}

	sharedBA, err := DeriveSharedSecret(privB[:], pubA[:])
	if err != nil {
		t.Fatalf("DeriveSharedSecret B→A: %v", err)
	}

	if len(sharedAB) != 32 {
		t.Errorf("expected 32-byte shared secret, got %d", len(sharedAB))
	}

	// Both sides should derive the same shared secret
	if string(sharedAB) != string(sharedBA) {
		t.Error("shared secrets from A and B should match")
	}
}

func TestEncryptDecryptPeer(t *testing.T) {
	privA, pubA, _ := GenerateDHKeyPair()
	privB, pubB, _ := GenerateDHKeyPair()

	sharedAB, _ := DeriveSharedSecret(privA[:], pubB[:])
	sharedBA, _ := DeriveSharedSecret(privB[:], pubA[:])

	plaintext := []byte("Hello, E2E encrypted world!")

	// A encrypts with sharedAB
	ciphertext, err := EncryptForPeer(sharedAB, plaintext)
	if err != nil {
		t.Fatalf("EncryptForPeer: %v", err)
	}

	// Ciphertext should be different from plaintext
	if string(ciphertext) == string(plaintext) {
		t.Error("ciphertext should not equal plaintext")
	}

	// B decrypts with sharedBA
	decrypted, err := DecryptFromPeer(sharedBA, ciphertext)
	if err != nil {
		t.Fatalf("DecryptFromPeer: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted text mismatch: got %q, want %q", string(decrypted), string(plaintext))
	}

	// Decrypting with wrong key should fail
	wrongPriv, _, _ := GenerateDHKeyPair()
	wrongShared, _ := DeriveSharedSecret(wrongPriv[:], pubA[:])
	_, err = DecryptFromPeer(wrongShared, ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong key, got nil")
	}
}
