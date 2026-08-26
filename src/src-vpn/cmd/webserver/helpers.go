package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/getsentry/sentry-go"
)

func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	handler := perUserLimiter.Middleware(next)
	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}

// ========== Security headers middleware ==========

func securityHeadersMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// SEC-001: CSP — block inline script exfil of nsec/localStorage keys.
		// 'unsafe-inline' kept for styles (UnoCSS runtime); scripts: self only.
		// connect-src allows same-origin API + WS + public STUN/Nostr relays.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"font-src 'self' data:; "+
				"connect-src 'self' ws: wss: http://127.0.0.1:* http://localhost:* http://10.*:* http://192.168.*:* https://*; "+
				"media-src 'self' blob:; "+
				"worker-src 'self' blob:; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'; "+
				"object-src 'none'")
		next(w, r)
	}
}

// ========== Cache headers for static assets ==========

func setCacheHeaders(w http.ResponseWriter, path string) {
	ext := filepath.Ext(path)
	switch ext {
	case ".js", ".css", ".png", ".jpg", ".jpeg", ".svg", ".woff", ".woff2":
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case ".html", ".json":
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	case ".wasm":
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
}

// ========== MIME type mapping ==========

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: failed to encode JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	// Capture 5xx errors to Sentry
	if status >= 500 {
		sentry.CaptureException(fmt.Errorf("HTTP %d [%s]: %s", status, code, message))
	}
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// ========== API Handlers ==========

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func containsPathTraversal(pathStr string) bool {
	return strings.Contains(pathStr, "..") || strings.Contains(pathStr, "\\")
}

func secureJoin(baseDir, targetFile string) (string, error) {
	cleanBase := filepath.Clean(baseDir)
	targetPath := filepath.Join(cleanBase, targetFile)
	cleanTarget := filepath.Clean(targetPath)

	if !strings.HasPrefix(cleanTarget, cleanBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal attempt")
	}
	return cleanTarget, nil
}

// ========== Server startup ==========
