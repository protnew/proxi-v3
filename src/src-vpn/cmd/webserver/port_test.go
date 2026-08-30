package main

import "testing"

func TestResolvePort_default8090(t *testing.T) {
	t.Setenv("PORT", "")
	if got := resolvePort(); got != "8090" {
		t.Fatalf("empty PORT = %q want 8090", got)
	}
}

func TestResolvePort_envWins(t *testing.T) {
	t.Setenv("PORT", "7777")
	if got := resolvePort(); got != "7777" {
		t.Fatalf("PORT=7777 got %q", got)
	}
}

func TestResolvePort_not8080Default(t *testing.T) {
	t.Setenv("PORT", "")
	if got := resolvePort(); got == "8080" {
		t.Fatal("empty PORT still 8080")
	}
}
