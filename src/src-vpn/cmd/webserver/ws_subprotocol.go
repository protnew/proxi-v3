package main

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

func jwtFromSubprotocol(r *http.Request) (string, error) {
	raw := r.Header.Get("Sec-WebSocket-Protocol")
	if raw == "" {
		return "", nil
	}
	var token string
	n := 0
	for _, part := range strings.Split(raw, ",") {
		s := strings.TrimSpace(part)
		if !strings.HasPrefix(s, "proxi-jwt.") {
			continue
		}
		n++
		token = strings.TrimPrefix(s, "proxi-jwt.")
	}
	if n > 1 {
		return "", errBadSubprotocol
	}
	if n == 0 {
		return "", nil
	}
	if token == "" || !subprotocolTokenOK(token) {
		return "", errBadSubprotocol
	}
	return token, nil
}

var errBadSubprotocol = errString("bad websocket subprotocol")

type errString string

func (e errString) Error() string { return string(e) }

func subprotocolTokenOK(tok string) bool {
	for _, r := range tok {
		if r > 127 || unicode.IsSpace(r) || strings.ContainsRune(" ,", r) {
			return false
		}
	}
	return true
}

func wsAcceptProtocols(r *http.Request) []string {
	raw := r.Header.Get("Sec-WebSocket-Protocol")
	if raw == "" {
		return nil
	}
	for _, part := range strings.Split(raw, ",") {
		if strings.TrimSpace(part) == "proxi" {
			return []string{"proxi"}
		}
	}
	return nil
}

func noteQueryTokenDeprecated(path string, r *http.Request) {
	if r.URL.Query().Get("token") == "" {
		return
	}
	log.Printf("ws query token deprecated path=%s", path)
}

func wsToken(r *http.Request) (string, error) {
	sub, err := jwtFromSubprotocol(r)
	if err != nil {
		return "", err
	}
	if sub != "" {
		return sub, nil
	}
	return r.URL.Query().Get("token"), nil
}

func acceptOriginPatterns(r *http.Request) []string {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return []string{"localhost"}
	}
	return []string{u.Hostname()}
}
