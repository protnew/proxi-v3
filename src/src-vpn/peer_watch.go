package vpn

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Peer-lost canary (BAG-49): probes the DONOR's own endpoint through the
// established chain — self-referential target, never a hardcoded public host.
// 2 consecutive fails -> peerLost -> reconnecting (attempt/max); after
// maxReconnect failed rounds -> error + reconnect_exhausted. Fail-closed:
// the kill-switch/WFP state is NEVER touched here — a dead donor leaves the
// engaged filters up so traffic cannot leak to physical egress.

const (
	watchInterval    = 5 * time.Second
	watchTimeout     = 3 * time.Second
	watchFailCount   = 2
	maxReconnectTries = 3
)

// watchPeer runs while state==connected to an exit donor. probe = TCP dial
// to the donor endpoint (its own egress listener address from the invite).
func (m *Manager) watchPeer(ctx context.Context, donorAddr string) {
	t := time.NewTicker(watchInterval)
	defer t.Stop()
	fails := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if m.GetStatus().State != StateConnected {
			return
		}
		if probeDonor(donorAddr) == nil {
			fails = 0
			continue
		}
		fails++
		if fails < watchFailCount {
			continue
		}
		m.mu.Lock()
		if !m.peerLost {
			m.peerLost = true
			m.state = StateReconnecting
			m.lastErr = "donor_offline"
			fmt.Printf("[VPN] donor_offline: %s unreachable, retrying\n", donorAddr)
		}
		m.mu.Unlock()

		// Reconnect rounds with simple backoff. WFP stays engaged (fail-closed).
		for attempt := 1; attempt <= maxReconnectTries; attempt++ {
			m.mu.Lock()
			m.reconnectAttempt = attempt
			m.mu.Unlock()
			if probeDonor(donorAddr) == nil {
				m.mu.Lock()
				m.peerLost = false
				m.reconnectAttempt = 0
				m.lastErr = ""
				m.state = StateConnected
				m.mu.Unlock()
				fmt.Printf("[VPN] donor back: %s\n", donorAddr)
				fails = 0
				goto RESUME
			}
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		m.mu.Lock()
		m.state = StateError
		m.lastErr = "reconnect_exhausted"
		m.reconnectAttempt = 0
		m.mu.Unlock()
		fmt.Printf("[VPN] reconnect_exhausted for donor %s — kill-switch stays engaged\n", donorAddr)
		return
	RESUME:
	}
}

func probeDonor(addr string) error {
	if addr == "" {
		return fmt.Errorf("no donor endpoint")
	}
	conn, err := net.DialTimeout("tcp", addr, watchTimeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
