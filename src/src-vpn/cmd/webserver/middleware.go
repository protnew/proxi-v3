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

		ctx := context.WithValue(r.Context(), "userID", claims.UserID)
		ctx = context.WithValue(ctx, "npub", claims.Npub)
		ctx = context.WithValue(ctx, "premium", claims.Premium)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// fileTokenOK — P15 (2026-09-20): файлы/медиа доступны по Bearer header ИЛИ
// ?token= JWT (нужен для <img src>/download-ссылок, где заголовок не поставить).
// Когда authService == nil (dev/test) — пропускает.
func (s *Server) fileTokenOK(r *http.Request) bool {
	if s.authService == nil {
		return true
	}
	tok := r.URL.Query().Get("token")
	if tok == "" {
		h := r.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(h), "bearer ") {
			tok = strings.TrimSpace(h[7:])
		}
	}
	if tok == "" {
		return false
	}
	_, err := s.authService.ValidateToken(tok)
	return err == nil
}
