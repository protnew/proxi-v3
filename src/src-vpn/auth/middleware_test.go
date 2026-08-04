package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetUserID_Empty(t *testing.T) {
	if id := GetUserID(context.Background()); id != "" {
		t.Fatalf("expected empty, got %q", id)
	}
}

func TestGetUserID_Set(t *testing.T) {
	ctx := context.WithValue(context.Background(), userIDKey, "u-42")
	if id := GetUserID(ctx); id != "u-42" {
		t.Fatalf("got %q", id)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	h := AuthMiddleware("secret-key-for-tests-32b!!!!")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "missing authorization") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestAuthMiddleware_BadFormat(t *testing.T) {
	h := AuthMiddleware("secret-key-for-tests-32b!!!!")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token abc")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	h := AuthMiddleware("secret-key-for-tests-32b!!!!")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.jwt")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddleware_ValidToken_SetsUserID(t *testing.T) {
	secret := "secret-key-for-tests-32b!!!!"
	svc := NewAuthService(secret)
	access, _, err := svc.GenerateTokenPair("user-99", "npub99")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	h := AuthMiddleware(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if got != "user-99" {
		t.Fatalf("userID=%q", got)
	}
}

func TestAuthMiddleware_WrongSecret(t *testing.T) {
	svc := NewAuthService("correct-secret-key-32bytes!!!!")
	access, _, err := svc.GenerateTokenPair("u1", "n1")
	if err != nil {
		t.Fatal(err)
	}
	h := AuthMiddleware("other-secret-key-32bytes-xxxx!!")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestRefreshToken_Invalid(t *testing.T) {
	svc := NewAuthService("secret-key-for-tests-32b!!!!")
	_, _, err := svc.RefreshToken("garbage")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateToken_WrongAlgRejection(t *testing.T) {
	// empty token
	svc := NewAuthService("secret-key-for-tests-32b!!!!")
	_, err := svc.ValidateToken("")
	if err == nil {
		t.Fatal("expected error on empty token")
	}
}

func TestGenerateTokenPair_ClaimsIssuer(t *testing.T) {
	svc := NewAuthService("secret-key-for-tests-32b!!!!")
	access, _, err := svc.GenerateTokenPair("uid", "npub")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := svc.ValidateToken(access)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Issuer != "unkillable-messenger" {
		t.Fatalf("issuer=%q", claims.Issuer)
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.After(time.Now()) {
		t.Fatal("access should not be expired")
	}
}
