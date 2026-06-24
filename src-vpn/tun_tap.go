package vpn

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// ========== Platform-specific (Linux/WSL stubs) ==========

// IsWireGuardAvailable checks whether the wg binary exists on the system.
func (m *Manager) IsWireGuardAvailable() bool {
	_, err := exec.LookPath("wg")
	return err == nil
}

// runCmd executes a command and captures its output for better error reporting
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return fmt.Errorf("%s: %w (output: %s)", name, err, string(out))
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func (m *Manager) setupInterface() error {
	// Stub mode: skip real interface setup
	if m.stubMode {
		fmt.Printf("[VPN] stub mode: skipping interface setup\n")
		m.myIP = "10.77.0.1"
		return nil
	}

	iface := m.config.InterfaceName
	ip := m.config.Address

	// ip link add um0 type wireguard
	if err := runCmd("ip", "link", "add", "dev", iface, "type", "wireguard"); err != nil {
		fmt.Printf("[VPN] Warning: ip link add: %v (may already exist)\n", err)
	}

	// wg set um0 private-key <key> listen-port 51820
	keyFile := filepath.Join(m.config.DataDir, "private.key")
	if err := runCmd("wg", "set", iface, "private-key", keyFile, "listen-port", fmt.Sprintf("%d", m.config.Port)); err != nil {
		return fmt.Errorf("wg set private-key: %w (is wireguard-tools installed?)", err)
	}

	// ip address add 10.77.0.1/24 dev um0
	if err := runCmd("ip", "address", "add", ip, "dev", iface); err != nil {
		fmt.Printf("[VPN] Warning: ip address add: %v (may already be assigned)\n", err)
	}

	// ip link set um0 up
	if err := runCmd("ip", "link", "set", iface, "up"); err != nil {
		return fmt.Errorf("ip link set up %s: %w", iface, err)
	}

	m.myIP = "10.77.0.1"
	return nil
}

func (m *Manager) teardownInterface() error {
	if m.stubMode {
		fmt.Printf("[VPN] stub mode: skipping teardownInterface\n")
		return nil
	}
	return runCmd("ip", "link", "del", m.config.InterfaceName)
}

func (m *Manager) addPeer(p *Peer) error {
	// Stub mode: skip real peer configuration
	if m.stubMode {
		fmt.Printf("[VPN] stub mode: skipping addPeer(%s)\n", p.PublicKey[:min(16, len(p.PublicKey))])
		return nil
	}

	args := []string{"set", m.config.InterfaceName, "peer", p.PublicKey, "allowed-ips", p.AllowedIPs}
	if p.Endpoint != "" {
		args = append(args, "endpoint", p.Endpoint)
	}
	if err := runCmd("wg", args...); err != nil {
		return fmt.Errorf("wg set peer %s: %w", p.PublicKey[:min(16, len(p.PublicKey))], err)
	}
	return nil
}

func (m *Manager) removePeerFromInterface(p *Peer) error {
	if m.stubMode {
		return nil
	}
	return runCmd("wg", "set", m.config.InterfaceName, "peer", p.PublicKey, "remove")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
