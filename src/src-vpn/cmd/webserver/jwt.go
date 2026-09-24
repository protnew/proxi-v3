package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

// requireJWTSecret refuses to start with an empty or hardcoded fallback secret.
func requireJWTSecret() (string, error) {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		return "", fmt.Errorf("JWT_SECRET is required")
	}
	switch s {
	case "change-me-in-production", "changeme", "secret", "password":
		return "", fmt.Errorf("JWT_SECRET is a known stub; set a unique value")
	}
	return s, nil
}

// perBootJWTSecret ignores a static .env secret unless JWT_SECRET_PIN=1.
// Each process start gets a fresh 32-byte secret so a local reader of .env
// cannot forge an admin JWT for this boot.
func perBootJWTSecret() (string, error) {
	if os.Getenv("JWT_SECRET_PIN") == "1" {
		return requireJWTSecret()
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("jwt boot secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
