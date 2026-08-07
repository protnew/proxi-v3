package vpn

// AmneziaWG DPI obfuscation module (Architecture table 02_DPI_Fallback, score 195).
//
// Phase 2 Desktop feature: AmneziaWG modifies WireGuard packets to evade DPI:
// - Junk packets before handshake
// - Modified header magic
// - Random transport padding
//
// In browser/PWA mode, DPI evasion is handled by WebRTC (looks like video call traffic).
// This stub provides the API surface for desktop (Tauri) integration.

import (
	"encoding/json"
	"fmt"
	"sync"
)

// AmneziaConfig holds DPI obfuscation parameters.
type AmneziaConfig struct {
	Enabled        bool   `json:"enabled"`
	// Obfuscation parameters (AmneziaWG-specific)
	JunkPacketCount  int    `json:"junkPacketCount,omitempty"`
	JunkPacketMinSize int   `json:"junkPacketMinSize,omitempty"`
	JunkPacketMaxSize int   `json:"junkPacketMaxSize,omitempty"`
	// Server endpoint (WireGuard server with AmneziaWG)
	Endpoint       string `json:"endpoint,omitempty"`
	// Pre-shared key for WireGuard
	PresharedKey   string `json:"-"` // never serialized to API
}

// DefaultAmneziaConfig returns optimal DPI evasion settings.
func DefaultAmneziaConfig() AmneziaConfig {
	return AmneziaConfig{
		Enabled:          false,
		JunkPacketCount:  5,
		JunkPacketMinSize: 8,
		JunkPacketMaxSize: 64,
	}
}

var (
	amneziaMu     sync.RWMutex
	amneziaCfg    = DefaultAmneziaConfig()
)

// GetAmneziaConfig returns current DPI obfuscation settings.
func GetAmneziaConfig() AmneziaConfig {
	amneziaMu.RLock()
	defer amneziaMu.RUnlock()
	return amneziaCfg
}

// SetAmneziaConfig updates DPI obfuscation settings.
func SetAmneziaConfig(cfg AmneziaConfig) error {
	if cfg.JunkPacketCount < 0 || cfg.JunkPacketCount > 20 {
		return fmt.Errorf("junkPacketCount must be 0-20, got %d", cfg.JunkPacketCount)
	}
	amneziaMu.Lock()
	defer amneziaMu.Unlock()
	amneziaCfg = cfg
	return nil
}

// AmneziaStatus returns JSON-serializable status for API.
func AmneziaStatus() map[string]interface{} {
	cfg := GetAmneziaConfig()
	return map[string]interface{}{
		"enabled":          cfg.Enabled,
		"junkPacketCount":  cfg.JunkPacketCount,
		"endpoint":         cfg.Endpoint,
		"phase":            "desktop_only",
		"note":             "DPI evasion for PWA uses WebRTC (looks like video call). AmneziaWG for desktop (Tauri) Phase 2.",
	}
}

// MarshalJSON for AmneziaConfig (omits PresharedKey).
func (c AmneziaConfig) MarshalJSON() ([]byte, error) {
	type Alias AmneziaConfig
	return json.Marshal(&struct {
		Alias
		PresharedKey string `json:"-"`
	}{
		Alias:        (Alias)(c),
		PresharedKey: "",
	})
}
