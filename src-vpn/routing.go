package vpn

import (
	"fmt"
	"net"
	"os"
	"time"
)

func (m *Manager) enableIPForwarding() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
}

func (m *Manager) setupNAT() error {
	// iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
	return runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-j", "MASQUERADE")
}

func (m *Manager) routeAllTraffic(endpoint string) error {
	// ip route add 0.0.0.0/0 dev um0
	runCmd("ip", "route", "add", "0.0.0.0/0", "dev", m.config.InterfaceName)
	return nil
}

// ==================== Split Tunneling ====================

// SplitTunnelConfig stores which domains/IPs go through VPN.
type SplitTunnelConfig struct {
	Mode    string   `json:"mode"`    // "all" (default), "split" (only listed), "exclude" (all except listed)
	Targets []string `json:"targets"` // domains or CIDR ranges
}

// SetSplitTunnel configures split tunneling rules via iptables.
func (m *Manager) SetSplitTunnel(cfg SplitTunnelConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up any previous split tunnel rules
	m.cleanupSplitTunnelLocked()

	// Create a fresh PROXI_SPLIT chain
	if err := runCmd("iptables", "-t", "mangle", "-N", "PROXI_SPLIT"); err != nil {
		fmt.Printf("[VPN] Warning: iptables PROXI_SPLIT creation failed: %v\n", err)
	}

	switch cfg.Mode {
	case "all":
		// No split rules — all traffic goes through VPN (default behavior)
		// Still jump to chain (it will just RETURN immediately)
	default:
		// "split" or "exclude"
		if cfg.Mode == "split" {
			// Only listed targets go through VPN
			for _, target := range cfg.Targets {
				if ip := net.ParseIP(target); ip != nil {
					runCmd("iptables", "-t", "mangle", "-A", "PROXI_SPLIT", "-d", target, "-j", "MARK", "--set-mark", "1")
				} else {
					// Resolve domain and add rule
					if addrs, err := net.LookupHost(target); err == nil {
						for _, addr := range addrs {
							runCmd("iptables", "-t", "mangle", "-A", "PROXI_SPLIT", "-d", addr, "-j", "MARK", "--set-mark", "1")
						}
					}
				}
			}
		} else if cfg.Mode == "exclude" {
			// All traffic through VPN except listed targets
			for _, target := range cfg.Targets {
				if ip := net.ParseIP(target); ip != nil {
					runCmd("iptables", "-t", "mangle", "-A", "PROXI_SPLIT", "-d", target, "-j", "RETURN")
				}
			}
			// Mark everything else (rules above RETURN first, so excluded targets skip this)
			runCmd("iptables", "-t", "mangle", "-A", "PROXI_SPLIT", "-j", "MARK", "--set-mark", "1")
		}
	}

	// Append unconditional RETURN at the end so unmatched traffic is not blocked
	runCmd("iptables", "-t", "mangle", "-A", "PROXI_SPLIT", "-j", "RETURN")

	// Connect the chain to PREROUTING (forwarded traffic) and OUTPUT (local traffic)
	runCmd("iptables", "-t", "mangle", "-A", "PREROUTING", "-j", "PROXI_SPLIT")
	runCmd("iptables", "-t", "mangle", "-A", "OUTPUT", "-j", "PROXI_SPLIT")

	return nil
}

// CleanupSplitTunnel removes all split tunneling iptables rules.
func (m *Manager) CleanupSplitTunnel() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cleanupSplitTunnelLocked()
}

// cleanupSplitTunnelLocked does the actual iptables cleanup (caller must hold m.mu).
func (m *Manager) cleanupSplitTunnelLocked() error {
	var firstErr error

	// Remove the jump rule from PREROUTING to PROXI_SPLIT
	if err := runCmd("iptables", "-t", "mangle", "-D", "PREROUTING", "-j", "PROXI_SPLIT"); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("delete PREROUTING jump: %w", err)
	}

	// Remove the jump rule from OUTPUT to PROXI_SPLIT
	if err := runCmd("iptables", "-t", "mangle", "-D", "OUTPUT", "-j", "PROXI_SPLIT"); err != nil && firstErr == nil {
		// Just log, don't override firstErr if it exists, to not mask other errors
		fmt.Printf("Warning: delete OUTPUT jump: %v\n", err)
	}

	// Flush all rules in the PROXI_SPLIT chain
	if err := runCmd("iptables", "-t", "mangle", "-F", "PROXI_SPLIT"); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("flush PROXI_SPLIT: %w", err)
	}

	// Delete the PROXI_SPLIT chain itself
	if err := runCmd("iptables", "-t", "mangle", "-X", "PROXI_SPLIT"); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("delete PROXI_SPLIT chain: %w", err)
	}

	return firstErr
}

// ==================== DNS through VPN ====================

// DNSConfig stores DNS proxy settings.
type DNSConfig struct {
	Enabled  bool   `json:"enabled"`
	Listen   string `json:"listen"`   // e.g. "127.0.0.1:5353"
	Upstream string `json:"upstream"` // e.g. "1.1.1.1:53"
}

// StartDNSProxy starts a simple DNS forwarder that routes queries through the WG tunnel.
func (m *Manager) StartDNSProxy(cfg DNSConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:5353"
	}
	if cfg.Upstream == "" {
		cfg.Upstream = "1.1.1.1:53"
	}

	addr, err := net.ResolveUDPAddr("udp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("DNS listen addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("DNS listen: %w", err)
	}

	go func() {
		defer conn.Close()
		buf := make([]byte, 512)
		for {
			n, clientAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}

			// Forward to upstream
			go func(query []byte, client *net.UDPAddr) {
				upstream, err := net.ResolveUDPAddr("udp", cfg.Upstream)
				if err != nil {
					return
				}
				upConn, err := net.DialUDP("udp", nil, upstream)
				if err != nil {
					return
				}
				defer upConn.Close()

				upConn.SetDeadline(time.Now().Add(5 * time.Second))
				upConn.Write(query)

				resp := make([]byte, 512)
				n, err := upConn.Read(resp)
				if err != nil {
					return
				}

				conn.WriteToUDP(resp[:n], client)
			}(append([]byte(nil), buf[:n]...), clientAddr)
		}
	}()

	return nil
}
