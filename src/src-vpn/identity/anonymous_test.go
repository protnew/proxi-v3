package identity

import (
	"testing"
	"time"
)

func TestGenerateDisposableIdentity(t *testing.T) {
	id, err := GenerateDisposableIdentity()
	if err != nil {
		t.Fatalf("GenerateDisposableIdentity: %v", err)
	}
	if id.PrivKey == nil {
		t.Error("privkey should not be nil")
	}
	if id.Npub == "" {
		t.Error("npub should not be empty")
	}
	if id.Nsec == "" {
		t.Error("nsec should not be empty")
	}
}

func TestDisposableIdentity_IsDisposable(t *testing.T) {
	id, _ := GenerateDisposableIdentity()
	if !id.IsDisposable() {
		t.Error("should be disposable (no mnemonic)")
	}
}

func TestDisposableIdentity_NotExpired(t *testing.T) {
	id, _ := GenerateDisposableIdentity()
	if id.IsExpired() {
		t.Error("should not be expired by default")
	}
}

func TestDisposableIdentity_ExpiresIn(t *testing.T) {
	id, _ := GenerateDisposableIdentity()
	id.ExpiresIn(1 * time.Second)
	if id.Expires == 0 {
		t.Error("expires should be set")
	}
	// Manually set to past to avoid timing issues
	id.Expires = time.Now().Unix() - 1
	if !id.IsExpired() {
		t.Error("should be expired")
	}
}

func TestDisposableIdentity_Uniqueness(t *testing.T) {
	id1, _ := GenerateDisposableIdentity()
	id2, _ := GenerateDisposableIdentity()
	if id1.Npub == id2.Npub {
		t.Error("two disposable identities should have different npubs")
	}
}
