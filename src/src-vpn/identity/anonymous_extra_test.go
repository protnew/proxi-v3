package identity

import (
	"bytes"
	"testing"
)

func TestGenerateDisposableIdentity_PubKey(t *testing.T) {
	id, err := GenerateDisposableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if id.PubKey == nil {
		t.Error("PubKey should not be nil")
	}
	// PubKey should match the private key's public key
	if !id.PubKey.IsEqual(id.PrivKey.PubKey()) {
		t.Error("PubKey should match PrivKey.PubKey()")
	}
}

func TestGenerateDisposableIdentity_CreatedTimestamp(t *testing.T) {
	id, err := GenerateDisposableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if id.Created == 0 {
		t.Error("Created timestamp should be non-zero")
	}
}

func TestGenerateDisposableIdentity_ExpiresDefault(t *testing.T) {
	id, err := GenerateDisposableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if id.Expires != 0 {
		t.Error("Expires should be 0 by default (never expires)")
	}
}

func TestDisposableIdentity_NpubFormat(t *testing.T) {
	id, err := GenerateDisposableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix([]byte(id.Npub), []byte("npub1")) {
		t.Errorf("Npub should start with 'npub1', got %q", id.Npub)
	}
}

func TestDisposableIdentity_NsecFormat(t *testing.T) {
	id, err := GenerateDisposableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix([]byte(id.Nsec), []byte("nsec1")) {
		t.Errorf("Nsec should start with 'nsec1', got %q", id.Nsec)
	}
}

func TestDisposableIdentity_Signing(t *testing.T) {
	id, err := GenerateDisposableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("test message")
	sig := Sign(id.PrivKey, data)
	if !Verify(id.PubKey, data, sig) {
		t.Error("signature verification should succeed for disposable identity")
	}
}

func TestDisposableIdentity_NpubRoundtrip(t *testing.T) {
	id, err := GenerateDisposableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := NpubToPubKey(id.Npub)
	if err != nil {
		t.Fatalf("NpubToPubKey: %v", err)
	}
	if !id.PubKey.IsEqual(recovered) {
		t.Error("Npub roundtrip failed")
	}
}

func TestDisposableIdentity_NsecRoundtrip(t *testing.T) {
	id, err := GenerateDisposableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := NsecToPrivKey(id.Nsec)
	if err != nil {
		t.Fatalf("NsecToPrivKey: %v", err)
	}
	origBytes := id.PrivKey.Serialize()
	recBytes := recovered.Serialize()
	for i := range origBytes {
		if origBytes[i] != recBytes[i] {
			t.Error("Nsec roundtrip failed: keys differ")
			break
		}
	}
}

func TestRandomBytes(t *testing.T) {
	b, err := RandomBytes(32)
	if err != nil {
		t.Fatalf("RandomBytes(32): %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(b))
	}
}

func TestRandomBytes_Uniqueness(t *testing.T) {
	b1, _ := RandomBytes(32)
	b2, _ := RandomBytes(32)
	match := true
	for i := range b1 {
		if b1[i] != b2[i] {
			match = false
			break
		}
	}
	if match {
		t.Error("two RandomBytes calls should produce different results")
	}
}

func TestRandomBytes_Zero(t *testing.T) {
	b, err := RandomBytes(0)
	if err != nil {
		t.Fatalf("RandomBytes(0): %v", err)
	}
	if len(b) != 0 {
		t.Errorf("expected 0 bytes, got %d", len(b))
	}
}

func TestRandomBytes_Large(t *testing.T) {
	b, err := RandomBytes(1024)
	if err != nil {
		t.Fatalf("RandomBytes(1024): %v", err)
	}
	if len(b) != 1024 {
		t.Fatalf("expected 1024 bytes, got %d", len(b))
	}
}

func TestDisposableIdentity_ExpiresIn_Future(t *testing.T) {
	id, _ := GenerateDisposableIdentity()
	// Set expiration far in the future
	id.ExpiresIn(3600e9) // 3600 seconds = 1 hour
	if id.IsExpired() {
		t.Error("should not be expired yet")
	}
}

func TestDisposableIdentity_IsExpired_ZeroMeansNever(t *testing.T) {
	id := &DisposableIdentity{Expires: 0}
	if id.IsExpired() {
		t.Error("Expires=0 should mean never expires")
	}
}
