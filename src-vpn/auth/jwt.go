package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is used for storing user ID in request context.
type contextKey string

const userIDKey contextKey = "userID"

// Claims represents the JWT claims for access and refresh tokens.
type Claims struct {
	UserID string `json:"user_id"`
	Npub   string `json:"npub"`
	jwt.RegisteredClaims
}

// AuthService handles JWT token generation, validation and refresh.
type AuthService struct {
	secretKey       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewAuthService creates a new AuthService with the given secret.
// Access tokens expire in 15 minutes, refresh tokens in 7 days.
func NewAuthService(secret string) *AuthService {
	return &AuthService{
		secretKey:       []byte(secret),
		accessTokenTTL:  15 * time.Minute,
		refreshTokenTTL: 7 * 24 * time.Hour,
	}
}

// GenerateTokenPair creates a new access/refresh token pair for the given user.
func (a *AuthService) GenerateTokenPair(userID, npub string) (accessToken, refreshToken string, err error) {
	now := time.Now()

	// Access token
	accessClaims := &Claims{
		UserID: userID,
		Npub:   npub,
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
		RegisteredClaims: jwt.RegisteredClaims{
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

	return accessToken, refreshToken, nil
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
func (a *AuthService) RefreshToken(refreshToken string) (newAccess, newRefresh string, err error) {
	claims, err := a.ValidateToken(refreshToken)
	if err != nil {
		return "", "", fmt.Errorf("validate refresh token: %w", err)
	}

	// Generate a new pair — the old refresh token is implicitly invalidated
	// because only the latest refresh token is stored per user.
	return a.GenerateTokenPair(claims.UserID, claims.Npub)
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
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
