package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is used for storing user ID in request context.
type contextKey string

const userIDKey contextKey = "userID"

// Claims represents the JWT claims for access and refresh tokens.
type Claims struct {
	UserID  string `json:"user_id"`
	Npub    string `json:"npub"`
	Premium bool   `json:"premium"`
	// Family identifies the refresh-token chain issued at login (P17).
	Family string `json:"fam,omitempty"`
	jwt.RegisteredClaims
}

// refreshEntry is the server-side state of one refresh token (P17).
type refreshEntry struct {
	userID    string
	family    string
	expiresAt time.Time
	rotated   bool // consumed by a later refresh
	revoked   bool // family revoked (reuse detected / logout)
}

// AuthService handles JWT token generation, validation and refresh.
type AuthService struct {
	secretKey       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration

	// refreshReg maps sha256(refresh token) → state (P17: rotation with
	// reuse detection). In-memory: a restart invalidates all refresh tokens
	// (fail-closed — users simply re-login).
	refreshMu  sync.Mutex
	refreshReg map[string]*refreshEntry
}

// NewAuthService creates a new AuthService with the given secret.
// Access tokens expire in 15 minutes, refresh tokens in 7 days.
func NewAuthService(secret string) *AuthService {
	return &AuthService{
		secretKey:       []byte(secret),
		accessTokenTTL:  15 * time.Minute,
		refreshTokenTTL: 7 * 24 * time.Hour,
		refreshReg:      make(map[string]*refreshEntry),
	}
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newTokenFamily() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fam-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// newJTI gives every token a unique ID: second-resolution iat alone can make
// two consecutive refresh tokens byte-identical, which would break rotation.
func newJTI() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("jti-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GenerateTokenPair creates a new access/refresh token pair for the given user
// and registers the refresh token under a fresh family (login).
func (a *AuthService) GenerateTokenPair(userID, npub string) (accessToken, refreshToken string, err error) {
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	return a.generatePairLocked(userID, npub, newTokenFamily())
}

// generatePairLocked signs a pair and registers the refresh token.
// Caller must hold refreshMu.
func (a *AuthService) generatePairLocked(userID, npub, family string) (accessToken, refreshToken string, err error) {
	now := time.Now()

	// Access token
	accessClaims := &Claims{
		UserID: userID,
		Npub:   npub,
		Family: family,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(a.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "unkillable-messenger",
			Subject:   userID,
		},
	}
	accessToken, err = a.signToken(accessClaims)
	if err != nil {
		return "", "", fmt.Errorf("sign access token: %w", err)
	}

	// Refresh token
	refreshClaims := &Claims{
		UserID: userID,
		Npub:   npub,
		Family: family,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        newJTI(),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "unkillable-messenger",
			Subject:   userID,
		},
	}
	refreshToken, err = a.signToken(refreshClaims)
	if err != nil {
		return "", "", fmt.Errorf("sign refresh token: %w", err)
	}

	// P17: register the refresh token (sha256, never the raw token).
	a.pruneExpiredLocked(now)
	a.refreshReg[hashRefreshToken(refreshToken)] = &refreshEntry{
		userID:    userID,
		family:    family,
		expiresAt: now.Add(a.refreshTokenTTL),
	}

	return accessToken, refreshToken, nil
}

func (a *AuthService) pruneExpiredLocked(now time.Time) {
	for h, e := range a.refreshReg {
		if now.After(e.expiresAt) {
			delete(a.refreshReg, h)
		}
	}
}

func (a *AuthService) revokeFamilyLocked(family string) {
	for _, e := range a.refreshReg {
		if e.family == family {
			e.revoked = true
		}
	}
}

// ValidateToken parses and validates a JWT token string, returning its claims.
func (a *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secretKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// RefreshToken validates a refresh token and issues a new token pair.
// P17: one-time refresh tokens with rotation and reuse detection — replaying
// an already-rotated token revokes the whole token family.
func (a *AuthService) RefreshToken(refreshToken string) (newAccess, newRefresh string, err error) {
	claims, err := a.ValidateToken(refreshToken)
	if err != nil {
		return "", "", fmt.Errorf("validate refresh token: %w", err)
	}

	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	now := time.Now()
	a.pruneExpiredLocked(now)

	entry, ok := a.refreshReg[hashRefreshToken(refreshToken)]
	if !ok {
		return "", "", errors.New("refresh token unknown, expired or issued before restart")
	}
	if entry.revoked {
		return "", "", errors.New("refresh token revoked")
	}
	if entry.rotated {
		// Reuse of a consumed token: steal-attempt — burn the whole family.
		a.revokeFamilyLocked(entry.family)
		return "", "", errors.New("refresh token reuse detected — token family revoked")
	}
	if entry.family != claims.Family || entry.userID != claims.UserID {
		return "", "", errors.New("refresh token claims mismatch")
	}

	entry.rotated = true
	access, refresh, gerr := a.generatePairLocked(claims.UserID, claims.Npub, entry.family)
	if gerr != nil {
		entry.rotated = false
		return "", "", gerr
	}
	return access, refresh, nil
}

// signToken signs a Claims struct into a JWT string.
func (a *AuthService) signToken(claims *Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secretKey)
}

// GetUserID extracts the UserID from the request context.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

// AuthMiddleware returns an HTTP middleware that extracts the Bearer token from
// the Authorization header, validates it, and sets the UserID in the request context.
// If the token is missing or invalid, it returns 401 Unauthorized.
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	svc := NewAuthService(secret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing authorization header"}}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid authorization header format"}}`, http.StatusUnauthorized)
				return
			}

			claims, err := svc.ValidateToken(parts[1])
			if err != nil {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid or expired token"}}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, "userID", claims.UserID)
			ctx = context.WithValue(ctx, "npub", claims.Npub)
			ctx = context.WithValue(ctx, "premium", claims.Premium)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
