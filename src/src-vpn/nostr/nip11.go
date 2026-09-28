// Package nostr implements the Nostr protocol.
//
// nip11.go implements NIP-11 Relay Information Document.
// Spec: https://github.com/nostr-protocol/nips/blob/master/11.md
//
// NIP-11 allows clients to discover relay capabilities, software version,
// supported NIPs, and administrative contact information via an HTTP GET
// to the relay URL with Accept: application/nostr+json header.
package nostr

// RelayInfoDoc is the NIP-11 Relay Information Document.
// It describes the relay's identity, capabilities, and administrative info.
type RelayInfoDoc struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Version       string `json:"version"`
	PubKey        string `json:"pubkey"`
	Software      string `json:"software"`
	Contact       string `json:"contact"`
	SupportedNIPs []int  `json:"supported_nips"`
}

// GetRelayInfoDoc returns the default relay information document for this server.
// In production, these values would be configurable via environment variables
// or a config file.
func GetRelayInfoDoc() RelayInfoDoc {
	return RelayInfoDoc{
		Name:          "unkillable-messenger-relay",
		Description:   "Unkillable Messenger Nostr relay — decentralized, censorship-resistant messaging",
		Version:       "1.0.0",
		PubKey:        "",
		Software:      "github.com/unkillable-messenger/vpn",
		Contact:       "admin@unkillable-messenger.example",
		SupportedNIPs: []int{1, 4, 5, 11, 28, 44},
	}
}
