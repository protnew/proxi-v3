package main

import (
	"net"
	"net/http"
	"strings"
)

// hostAllowed is the F10 DNS-rebinding gate: only loopback names.
func hostAllowed(hostport string) bool {
	if hostport == "" {
		return false
	}
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost":
		return true
	default:
		return false
	}
}

func hostGate(next http.Handler) http.Handler {
	if next == nil {
		next = http.DefaultServeMux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host) {
			http.Error(w, `{"error":"bad host"}`, http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
