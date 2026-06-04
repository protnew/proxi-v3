// Package nostr implements the Nostr protocol.
//
// nip05.go implements NIP-05 DNS-based internet identifiers.
// Spec: https://github.com/nostr-protocol/nips/blob/master/05.md
//
// NIP-05 maps human-readable identifiers (user@domain.com) to Nostr public keys
// via a well-known JSON endpoint: GET https://domain/.well-known/nostr.json?name=user
package nostr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// nip05Response is the JSON response from a NIP-05 lookup.
type nip05Response struct {
	Names  map[string]string `json:"names"`
	Relays map[string][]string `json:"relays,omitempty"`
}

// VerifyNIP05 verifies a NIP-05 DNS identity (user@domain.com).
// It fetches the well-known JSON endpoint and checks if the provided npub (hex)
// matches the one returned by the DNS server.
//
// Parameters:
//   - npub: hex-encoded public key of the user
//   - identifier: NIP-05 identifier in the form "user@domain.com"
//
// Returns true if the identifier maps to the given npub.
func VerifyNIP05(npub, identifier string) (bool, error) {
	parts := strings.SplitN(identifier, "@", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("nip05: invalid identifier format, expected user@domain.com")
	}
	name := parts[0]
	domain := parts[1]

	url := fmt.Sprintf("https://%s/.well-known/nostr.json?name=%s", domain, name)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, fmt.Errorf("nip05: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("nip05: HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("nip05: read body: %w", err)
	}

	var result nip05Response
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("nip05: decode JSON: %w", err)
	}

	// Normalize npub to check against both possible representations
	registeredPubKey, ok := result.Names[name]
	if !ok {
		return false, nil
	}

	// Try case-insensitive match (pubkeys are hex, so case doesn't matter)
	if strings.EqualFold(registeredPubKey, npub) {
		return true, nil
	}

	return false, nil
}
