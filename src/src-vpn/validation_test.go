package vpn

import (
	"strings"
	"testing"
)

func TestValidateSignupInput(t *testing.T) {
	// OK
	if err := ValidateSignupInput("Alice", strings.Repeat("a", 64)); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	// Name too long
	if err := ValidateSignupInput(strings.Repeat("x", 101), "pubkey"); err == nil {
		t.Fatal("name >100 should fail")
	}
	// PubKey too long
	if err := ValidateSignupInput("Bob", strings.Repeat("k", 129)); err == nil {
		t.Fatal("npub >128 should fail")
	}
}

func TestValidateMessage(t *testing.T) {
	// OK
	if err := ValidateMessage("hello"); err != nil {
		t.Fatalf("short message rejected: %v", err)
	}
	// Too long
	long := strings.Repeat("x", MaxMessageLen+1)
	if err := ValidateMessage(long); err == nil {
		t.Fatal("message >16KB should fail")
	}
}
