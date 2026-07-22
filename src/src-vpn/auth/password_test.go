package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if hash == "secret123" {
		t.Error("hash should not be plaintext")
	}
}

func TestCheckPassword(t *testing.T) {
	hash, _ := HashPassword("mypassword")
	if !CheckPassword("mypassword", hash) {
		t.Error("correct password should match")
	}
	if CheckPassword("wrongpassword", hash) {
		t.Error("wrong password should not match")
	}
}

func TestHashPassword_Empty(t *testing.T) {
	_, err := HashPassword("")
	if err == nil {
		t.Error("empty password should fail")
	}
}

func TestHashPassword_TooLong(t *testing.T) {
	longPass := make([]byte, 73)
	for i := range longPass {
		longPass[i] = 'a'
	}
	_, err := HashPassword(string(longPass))
	if err == nil {
		t.Error("72+ char password should fail")
	}
}

func TestHashPassword_DifferentHashes(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("same password should produce different hashes (salt)")
	}
}
