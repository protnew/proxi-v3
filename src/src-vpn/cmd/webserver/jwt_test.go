package main

import "testing"

func TestRequireJWTSecret_emptyFails(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, err := requireJWTSecret(); err == nil {
		t.Fatal("empty JWT_SECRET must fail")
	}
}

func TestRequireJWTSecret_envWins(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-only")
	got, err := requireJWTSecret()
	if err != nil {
		t.Fatal(err)
	}
	if got != "unit-test-only" {
		t.Fatalf("got %q", got)
	}
}

func TestRequireJWTSecret_noFallbackLiteral(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	_, err := requireJWTSecret()
	if err == nil {
		t.Fatal("wanted error")
	}
	if err.Error() != "JWT_SECRET is required" {
		t.Fatalf("err = %v", err)
	}
}
