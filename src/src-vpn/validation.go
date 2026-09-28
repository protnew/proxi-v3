package vpn

import "fmt"

// Input validation limits (SEC-002)
const (
	MaxNameLen       = 100
	MaxMessageLen    = 16384 // 16 KB per message
	MaxPubKeyLen     = 128
	MaxEndpointLen   = 256
	MaxChatTitleLen  = 200
)

// ValidateSignupInput checks name and pubkey length.
func ValidateSignupInput(name, npub string) error {
	if len(name) > MaxNameLen {
		return fmt.Errorf("name too long: %d > %d", len(name), MaxNameLen)
	}
	if len(npub) > MaxPubKeyLen {
		return fmt.Errorf("npub too long: %d > %d", len(npub), MaxPubKeyLen)
	}
	return nil
}

// ValidateMessage checks message text length.
func ValidateMessage(text string) error {
	if len(text) > MaxMessageLen {
		return fmt.Errorf("message too long: %d > %d", len(text), MaxMessageLen)
	}
	return nil
}
