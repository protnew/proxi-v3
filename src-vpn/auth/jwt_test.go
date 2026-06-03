package auth

import (
	"testing"
	"time"
)

func TestGenerateTokenPair(t *testing.T) {
	svc := NewAuthService("test-secret-key-32bytes-long!!")

	access, refresh, err := svc.GenerateTokenPair("user-123", "npub1abc")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	if access == "" {
		t.Error("access token is empty")
	}
	if refresh == "" {
		t.Error("refresh token is empty")
	}
	if access == refresh {
		t.Error("access and refresh tokens should differ")
	}
}

func TestValidateToken(t *testing.T) {
	svc := NewAuthService("test-secret-key-32bytes-long!!")

	access, _, err := svc.GenerateTokenPair("user-123", "npub1abc")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}

	claims, err := svc.ValidateToken(access)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected UserID 'user-123', got '%s'", claims.UserID)
	}
	if claims.Npub != "npub1abc" {
		t.Errorf("expected Npub 'npub1abc', got '%s'", claims.Npub)
	}
}

func TestExpiredToken(t *testing.T) {
	svc := NewAuthService("test-secret-key-32bytes-long!!")
	// Override TTLs to make tokens expire immediately
	svc.accessTokenTTL = -1 * time.Second

	access, _, err := svc.GenerateTokenPair("user-123", "npub1abc")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}

	_, err = svc.ValidateToken(access)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestRefreshToken(t *testing.T) {
	svc := NewAuthService("test-secret-key-32bytes-long!!")

	_, refresh, err := svc.GenerateTokenPair("user-456", "npub1def")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}

	newAccess, newRefresh, err := svc.RefreshToken(refresh)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if newAccess == "" {
		t.Error("new access token is empty")
	}
	if newRefresh == "" {
		t.Error("new refresh token is empty")
	}

	// Validate the new access token
	claims, err := svc.ValidateToken(newAccess)
	if err != nil {
		t.Fatalf("ValidateToken (new access): %v", err)
	}
	if claims.UserID != "user-456" {
		t.Errorf("expected UserID 'user-456', got '%s'", claims.UserID)
	}

	// Validate the new refresh token
	claims2, err := svc.ValidateToken(newRefresh)
	if err != nil {
		t.Fatalf("ValidateToken (new refresh): %v", err)
	}
	if claims2.UserID != "user-456" {
		t.Errorf("expected UserID 'user-456', got '%s'", claims2.UserID)
	}
}

func TestInvalidToken(t *testing.T) {
	svc := NewAuthService("test-secret-key-32bytes-long!!")

	// Completely invalid string
	_, err := svc.ValidateToken("not-a-valid-token")
	if err == nil {
		t.Error("expected error for invalid token, got nil")
	}

	// Token signed with wrong secret
	svc2 := NewAuthService("different-secret-key-32-bytes!")
	access, _, _ := svc.GenerateTokenPair("user-123", "npub1abc")
	_, err = svc2.ValidateToken(access)
	if err == nil {
		t.Error("expected error for wrong-secret token, got nil")
	}

	// Empty token
	_, err = svc.ValidateToken("")
	if err == nil {
		t.Error("expected error for empty token, got nil")
	}
}
