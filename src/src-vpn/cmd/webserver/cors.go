package main

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// corsAllowedOrigins returns the whitelist of origins permitted to make
// credentialed cross-origin requests. The list is read from the
// CORS_ALLOWED_ORIGINS environment variable (comma-separated). When the
// variable is empty, a permissive development default is used.
//
// VPN-LAN-001: also allows private-network origins (10/8, 172.16/12, 192.168/16, localhost)
// so a phone on the same Wi‑Fi can open http://LAN_IP:8090 without CORS breakage on API.
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
			"http://localhost:8090",
			"http://127.0.0.1:8090",
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

func isPrivateLANOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	// RFC1918 + link-local
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"fc00::/7",
		"fe80::/10",
	}
	for _, c := range privateCIDRs {
		_, netw, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		if netw.Contains(ip) {
			return true
		}
	}
	return false
}

// applyCORSHeaders sets Access-Control-* headers on w based on the request
// Origin. Returns true when the origin was allowed.
func applyCORSHeaders(w http.ResponseWriter, r *http.Request, allowCredentials bool) bool {
	origin := r.Header.Get("Origin")
	allowed := false
	for _, o := range corsAllowedOrigins() {
		if o == origin {
			allowed = true
			break
		}
	}
	if !allowed && isPrivateLANOrigin(origin) {
		allowed = true
	}

	if origin == "" && !allowCredentials {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		return true
	}

	if !allowed {
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
