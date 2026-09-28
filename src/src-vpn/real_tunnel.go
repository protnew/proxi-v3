package vpn

import (
	"fmt"
	"time"
)

// StartRealTunnel starts a REAL local SOCKS5 proxy that carries app traffic.
// listen: e.g. "127.0.0.1:10808". upstreamSOCKS optional "host:port" for exit chaining.
// This does NOT rewrite Windows system routes; configure browser/app to use the SOCKS address.
func (m *Manager) StartRealTunnel(listen, upstreamSOCKS string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.socks != nil && m.socks.IsRunning() {
		m.state = StateConnected
		m.mode = "real"
		return nil
	}
	if listen == "" {
		listen = "127.0.0.1:10808"
	}
	m.state = StateConnecting
	srv := NewSOCKS5Server(listen, upstreamSOCKS)
	if err := srv.Start(); err != nil {
		m.state = StateError
		return err
	}
	// stop previous if any
	if m.socks != nil {
		_ = m.socks.Stop()
	}
	m.socks = srv
	m.myIP = "127.0.0.1"
	m.startTime = time.Now()
	m.mode = "real"
	if upstreamSOCKS != "" {
		m.mode = "exit"
	}
	m.stubMode = false
	m.state = StateConnected
	fmt.Printf("[VPN] REAL tunnel SOCKS5 on %s upstream=%q\n", listen, upstreamSOCKS)
	return nil
}

// CheckEgressIP fetches a public IP via the active SOCKS5 (proof of real traffic path).
func (m *Manager) CheckEgressIP() (string, error) {
	m.mu.RLock()
	socks := m.socks
	m.mu.RUnlock()
	if socks == nil || !socks.IsRunning() {
		return "", fmt.Errorf("real SOCKS tunnel not running — start real tunnel first")
	}
	// Dial api.ipify.org:80 through our SOCKS and do a minimal HTTP GET
	conn, err := dialViaSOCKS5(socks.Addr(), "api.ipify.org:80", 15*time.Second)
	if err != nil {
		return "", fmt.Errorf("socks dial: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	req := "GET / HTTP/1.1\r\nHost: api.ipify.org\r\nConnection: close\r\nUser-Agent: proxi-vpn-check\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		return "", err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil && n == 0 {
		return "", err
	}
	body := string(buf[:n])
	// body after headers
	idx := -1
	for i := 0; i+3 < len(body); i++ {
		if body[i] == '\r' && body[i+1] == '\n' && body[i+2] == '\r' && body[i+3] == '\n' {
			idx = i + 4
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("bad http response")
	}
	ip := ""
	for _, ch := range body[idx:] {
		if (ch >= '0' && ch <= '9') || ch == '.' || (ch >= 'a' && ch <= 'f') || ch == ':' {
			ip += string(ch)
		} else if len(ip) > 0 {
			break
		}
	}
	if ip == "" {
		return "", fmt.Errorf("empty ip body")
	}
	return ip, nil
}

