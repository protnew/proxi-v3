package tor

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// TorDialer creates connections through Tor SOCKS5 proxy.
type TorDialer struct {
	proxyAddr string // e.g. "127.0.0.1:9050"
}

// NewTorDialer creates a dialer that routes through Tor.
func NewTorDialer(proxyAddr string) *TorDialer {
	if proxyAddr == "" {
		proxyAddr = "127.0.0.1:9050"
	}
	return &TorDialer{proxyAddr: proxyAddr}
}

// Dial connects to an .onion address through Tor SOCKS5 proxy.
func (d *TorDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

// DialContext connects with context through Tor.
func (d *TorDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// Use Tor's SOCKS5 proxy
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", d.proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("connect to Tor proxy %s: %w", d.proxyAddr, err)
	}

	// SOCKS5 handshake
	// 1. Greeting: no auth
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 greeting: %w", err)
	}

	buf := make([]byte, 2)
	if _, err := conn.Read(buf); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 greeting response: %w", err)
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("socks5 unexpected response: %x", buf)
	}

	// 2. Connect request
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("parse address %s: %w", addr, err)
	}

	portNum := 0
	fmt.Sscanf(port, "%d", &portNum)

	// SOCKS5 connect: version=5, cmd=connect(1), rsv=0, atyp=domain(3)
	req := []byte{0x05, 0x01, 0x00, 0x03}
	req = append(req, byte(len(host)))
	req = append(req, []byte(host)...)
	req = append(req, byte(portNum>>8), byte(portNum&0xff))

	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect request: %w", err)
	}

	// Read response (min 10 bytes for domain)
	resp := make([]byte, 256)
	n, err := conn.Read(resp)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect response: %w", err)
	}
	if n < 4 || resp[1] != 0x00 {
		conn.Close()
		status := "unknown"
		if n >= 2 {
			switch resp[1] {
			case 0x01:
				status = "general failure"
			case 0x04:
				status = "host unreachable"
			case 0x05:
				status = "connection refused"
			default:
				status = fmt.Sprintf("error code %d", resp[1])
			}
		}
		return nil, fmt.Errorf("socks5 connect failed: %s", status)
	}

	return conn, nil
}

// HTTPTransport returns an HTTP transport that routes through Tor.
func (d *TorDialer) HTTPTransport() *http.Transport {
	return &http.Transport{
		DialContext: d.DialContext,
	}
}

// HTTPClient returns an HTTP client that routes through Tor.
func (d *TorDialer) HTTPClient() *http.Client {
	return &http.Client{
		Transport: d.HTTPTransport(),
		Timeout:   60 * time.Second,
	}
}

// IsTorRunning checks if the Tor SOCKS5 proxy is available.
func (d *TorDialer) IsTorRunning() bool {
	conn, err := net.DialTimeout("tcp", d.proxyAddr, 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// OnionService represents a Tor hidden service.
type OnionService struct {
	OnionAddr string `json:"onionAddr"`
	Port      int    `json:"port"`
}

// GetOnionAddress reads the .onion address from Tor control port or filesystem.
// Tries control port first (port 9051), then falls back to hostname file.
func GetOnionAddress(dataDir string) string {
	// Try control port (9051)
	if addr, err := getOnionFromControlPort("127.0.0.1:9051"); err == nil && addr != "" {
		return addr
	}
	// Fallback: read from Tor data directory
	if dataDir == "" {
		dataDir = "/var/lib/tor"
	}
	// Common hidden service locations
	paths := []string{
		dataDir + "/proxi/hostname",
		dataDir + "/hidden_service/hostname",
		"/var/lib/tor/proxi/hostname",
		"/var/lib/tor/hidden_service/hostname",
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			addr := strings.TrimSpace(string(data))
			if addr != "" {
				return addr
			}
		}
	}
	return "not-yet-configured.onion"
}

// getOnionFromControlPort queries Tor control port for hidden service addresses.
func getOnionFromControlPort(addr string) (string, error) {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Authenticate (no password — cookie or empty)
	conn.Write([]byte("AUTHENTICATE \"\"\r\n"))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil || !bytes.Contains(buf[:n], []byte("250")) {
		return "", fmt.Errorf("auth failed")
	}

	// Get hidden service descriptors
	conn.Write([]byte("GETINFO onions/current\r\n"))
	n, err = conn.Read(buf)
	if err != nil {
		return "", err
	}

	// Parse response: 250-ons/current=abcdef1234.onion
	lines := string(buf[:n])
	for _, line := range strings.Split(lines, "\n") {
		if strings.HasPrefix(line, "250-ons/current=") {
			addr := strings.TrimPrefix(line, "250-ons/current=")
			addr = strings.TrimSpace(addr)
			if addr != "" && strings.HasSuffix(addr, ".onion") {
				return addr, nil
			}
		}
	}

	return "", fmt.Errorf("no onion address found")
}

// CreateOnionService creates a new hidden service via Tor control port.
func CreateOnionService(controlAddr string, targetPort int) (*OnionService, error) {
	if controlAddr == "" {
		controlAddr = "127.0.0.1:9051"
	}
	if targetPort <= 0 {
		targetPort = 9999
	}

	conn, err := net.DialTimeout("tcp", controlAddr, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to Tor control port: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	// Authenticate
	conn.Write([]byte("AUTHENTICATE \"\"\r\n"))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || !bytes.Contains(buf[:n], []byte("250")) {
		return nil, fmt.Errorf("Tor auth failed")
	}

	// Create ephemeral hidden service
	cmd := fmt.Sprintf("ADD_ONION NEW:BEST Flags=DiscardPK Port=80,127.0.0.1:%d\r\n", targetPort)
	conn.Write([]byte(cmd))

	n, err = conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("ADD_ONION read: %w", err)
	}

	resp := string(buf[:n])
	var onionAddr string
	for _, line := range strings.Split(resp, "\n") {
		if strings.HasPrefix(line, "250-ServiceID=") {
			onionAddr = strings.TrimPrefix(line, "250-ServiceID=")
			onionAddr = strings.TrimSpace(onionAddr) + ".onion"
		}
	}
	if onionAddr == "" {
		return nil, fmt.Errorf("no ServiceID in response: %s", resp)
	}

	return &OnionService{
		OnionAddr: onionAddr,
		Port:      80,
	}, nil
}
