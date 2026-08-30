package main

import (
	"fmt"
	"os"
)

// requireJWTSecret refuses to start with an empty or hardcoded fallback secret.
func requireJWTSecret() (string, error) {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		return "", fmt.Errorf("JWT_SECRET is required")
	}
	return s, nil
}
