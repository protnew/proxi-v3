package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/unkillable-messenger/vpn/auth"
)

// authMiddleware enforces JWT authentication on protected routes.
func authMiddleware(authSvc *auth.AuthService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing Authorization header")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid Authorization header format")
			return
		}

		claims, err := authSvc.ValidateToken(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")
			return
		}

		// Inject userID and npub into context
		ctx := context.WithValue(r.Context(), "userID", claims.UserID)
		ctx = context.WithValue(ctx, "npub", claims.Npub)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
