// DK-002: Desktop WireGuard system VPN — Tauri command interface.
// This file documents the WireGuard integration for Tauri desktop app.
// Real implementation lives in src-tauri/src/wireguard.rs (Rust).
package main

// WireGuardConfig represents a WireGuard tunnel configuration.
type WireGuardConfig struct {
	PrivateKey  string
	Address     string
	DNS         []string
	Peer        WireGuardPeer
}

type WireGuardPeer struct {
	PublicKey  string
	Endpoint   string
	AllowedIPs []string
}

// Tauri commands (invoked from frontend):
// invoke('wg_connect', { config })
// invoke('wg_disconnect')
// invoke('wg_status')
