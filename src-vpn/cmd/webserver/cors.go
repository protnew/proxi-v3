package main

import (
	"net/http"
	"os"
	"strings"
)

// corsAllowedOrigins returns the whitelist of origins permitted to make
// credentialed cross-origin requests. The list is read from the
// CORS_ALLOWED_ORIGINS environment variable (comma-separated). When the
// variable is empty, a permissive development default is used:
// localhost on ports 5173/5174/4173 (Vite dev/preview) and 8080 (Go API).
//
// P0-3 RESCUE 20260720: replaces the previous origin-reflection behavior
// that leaked credentials (Allow-Credentials: true) to any caller.
func corsAllowedOrigins() []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		return []string{
			"http://localhost:5173",
			"http://localhost:5174",
			"http://localhost:4173",
			"http://127.0.0.1:5173",
			"http://127.0.0.1:5174",
			"http://127.0.0.1:4173",
			"http://localhost:8080",
			"http://127.0.0.1:8080",
		}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// applyCORSHeaders sets Access-Control-* headers on w based on the request
// Origin. When the origin is not in the whitelist, NO ACAO header is set
// and the caller should treat the request as same-origin (or reject it).
// Returns true when the origin was allowed.
//
// When credentials are NOT required (allowCredentials=false), a wildcard
// ACAO "*" is emitted for non-browser clients (Origin absent or empty).
func applyCORSHeaders(w http.ResponseWriter, r *http.Request, allowCredentials bool) bool {
	origin := r.Header.Get("Origin")
	allowed := false
	for _, o := range corsAllowedOrigins() {
		if o == origin {
			allowed = true
			break
		}
	}

	if origin == "" && !allowCredentials {
		// Same-origin / non-browser: allow with wildcard.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		return true
	}

	if !allowed {
		// Do NOT emit Access-Control-Allow-Origin. Browser will block.
		return false
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	if allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept")
	w.Header().Set("Access-Control-Max-Age", "86400")
	return true
}
