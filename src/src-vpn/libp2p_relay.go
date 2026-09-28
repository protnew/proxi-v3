package vpn

// libp2p Circuit Relay - Phase 2 Desktop (Architecture table 57, score 191).
//
// libp2p provides:
// - Kademlia DHT for peer discovery
// - AutoNAT for NAT type detection
// - DCUtR (Direct Connection Upgrade through Relay) for hole punching
// - Circuit Relay v2 for relay when P2P fails
// - Noise protocol for E2E encryption
//
// In browser/PWA mode, we use WebRTC + STUN + Nostr relay.
// This module provides the API surface for desktop (Tauri) integration.
//
// Dependencies (when Phase 2 starts):
//   go get github.com/libp2p/go-libp2p
//   go get github.com/libp2p/go-libp2p-kad-dht
//   go get github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Libp2pConfig holds the circuit relay configuration.
type Libp2pConfig struct {
	Enabled           bool     `json:"enabled"`
	// Listen addresses for the libp2p host
	ListenAddrs       []string `json:"listenAddrs,omitempty"`
	// Bootstrap peers for DHT (default: IPFS bootstrap nodes)
	BootstrapPeers    []string `json:"bootstrapPeers,omitempty"`
	// Relay peers for circuit relay fallback
	RelayPeers        []string `json:"relayPeers,omitempty"`
	// Enable relay service (allow other peers to use us as relay)
	RelayService      bool     `json:"relayService"`
	// Enable DHT (Kademlia distributed hash table)
	DHTEnabled        bool     `json:"dhtEnabled"`
	// Enable DCUtR (Direct Connection Upgrade through Relay)
	DCUtREnabled      bool     `json:"dcutrEnabled"`
}

// DefaultLibp2pConfig returns optimal settings for desktop relay.
func DefaultLibp2pConfig() Libp2pConfig {
	return Libp2pConfig{
		Enabled:      false,
		ListenAddrs:  []string{"/ip4/0.0.0.0/tcp/0", "/ip4/0.0.0.0/udp/0/quic"},
		RelayService: true,  // allow this node to relay for others
		DHTEnabled:   true,  // participate in DHT
		DCUtREnabled: true,  // try direct connection upgrade
		BootstrapPeers: []string{
			// IPFS bootstrap nodes (public, free, decentralized)
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDuVkcruPhcoXDia1Zc1tksTjzFqEaR7XWvK2mDWe",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uTSa",
		},
	}
}

var (
	libp2pMu  sync.RWMutex
	libp2pCfg = DefaultLibp2pConfig()
)

// GetLibp2pConfig returns current libp2p configuration.
func GetLibp2pConfig() Libp2pConfig {
	libp2pMu.RLock()
	defer libp2pMu.RUnlock()
	return libp2pCfg
}

// SetLibp2pConfig updates libp2p configuration.
func SetLibp2pConfig(cfg Libp2pConfig) error {
	libp2pMu.Lock()
	defer libp2pMu.Unlock()
	libp2pCfg = cfg
	return nil
}

// Libp2pStatus returns JSON-serializable status for API.
func Libp2pStatus() map[string]interface{} {
	cfg := GetLibp2pConfig()
	return map[string]interface{}{
		"enabled":         cfg.Enabled,
		"phase":           "desktop_only",
		"relayService":    cfg.RelayService,
		"dht":             cfg.DHTEnabled,
		"dcutr":           cfg.DCUtREnabled,
		"bootstrapPeers":  len(cfg.BootstrapPeers),
		"listenAddrs":     cfg.ListenAddrs,
		"note":            "libp2p circuit relay for Desktop (Tauri). Browser uses WebRTC + Nostr relay.",
		"table":           "57_Global_P2P_Transport (Score 191)",
	}
}

// MarshalJSON for Libp2pConfig.
func (c Libp2pConfig) MarshalJSON() ([]byte, error) {
	type Alias Libp2pConfig
	return json.Marshal((Alias)(c))
}

// Validate checks the configuration for errors.
func (c *Libp2pConfig) Validate() error {
	if c.Enabled && len(c.ListenAddrs) == 0 {
		return fmt.Errorf("libp2p enabled but no listen addresses")
	}
	return nil
}
