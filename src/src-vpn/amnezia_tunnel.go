package vpn

// amnezia_tunnel.go — DPI-001 real tunnel lifecycle (desktop).
// Generates X25519 keys, writes AmneziaWG conf, applies via local driver if present.
// Without kernel driver: status=driver_missing but conf+keys are production-ready.

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// TunnelStatus is JSON for /api/vpn/amnezia/tunnel
type TunnelStatus struct {
	State       string `json:"state"` // stopped|starting|up|error|driver_missing
	Mode        string `json:"mode"`  // kernel|userspace|none
	Driver      string `json:"driver,omitempty"`
	ConfPath    string `json:"confPath,omitempty"`
	Interface   string `json:"interface,omitempty"`
	LocalPub    string `json:"localPublicKey,omitempty"`
	PeerPub     string `json:"peerPublicKey,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Address     string `json:"address,omitempty"`
	StartedAt   int64  `json:"startedAt,omitempty"`
	LastError   string `json:"lastError,omitempty"`
	Jc          int    `json:"jc,omitempty"`
	Note        string `json:"note,omitempty"`
}

// TunnelStartRequest from API
type TunnelStartRequest struct {
	PeerPublicKey string `json:"peerPublicKey"`
	Endpoint      string `json:"endpoint"`
	AllowedIPs    string `json:"allowedIPs,omitempty"`
	Address       string `json:"address,omitempty"`
	// Optional: provide private key (base64 WG). Empty = generate.
	PrivateKey string `json:"privateKey,omitempty"`
	PeerEndpointPresharedKey string `json:"presharedKey,omitempty"`
}

type tunnelRuntime struct {
	mu     sync.RWMutex
	status TunnelStatus
	// private key kept only in memory + conf file (0600)
	privateKeyB64 string
	confPath      string
	cmd           *exec.Cmd
	userspace     *UserspaceVPN
}

var tunnel = &tunnelRuntime{
	status: TunnelStatus{
		State: "stopped",
		Mode:  "none",
		Note:  "In-app userspace VPN is primary. Kernel/AmneziaVPN GUI optional.",
	},
}

// GenerateWGKeyPair returns (privateB64, publicB64) WireGuard-style.
func GenerateWGKeyPair() (privB64, pubB64 string, err error) {
	priv, pub, err := GenerateDHKeyPairFromVPN()
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv[:]), base64.StdEncoding.EncodeToString(pub[:]), nil
}

// GenerateDHKeyPairFromVPN uses crypto package X25519 via thin wrapper in this package.
// Implemented in amnezia_tunnel_keys.go to avoid import cycle with crypto/.

func dataDir() string {
	if d := os.Getenv("DATA_DIR"); d != "" {
		return d
	}
	if d := os.Getenv("IND_DATA"); d != "" {
		return d
	}
	return filepath.Join(".", "data")
}

func amneziaDir() string {
	d := filepath.Join(dataDir(), "amnezia")
	_ = os.MkdirAll(d, 0o700)
	return d
}

// DetectTunnelDriver finds awg/wireguard binaries.
func DetectTunnelDriver() (name, path string) {
	candidates := []struct{ name, path string }{
		{"amneziawg", ""},
		{"awg", ""},
		{"awg-quick", ""},
		{"wireguard", ""},
		{"wg-quick", ""},
		{"wg", ""},
	}
	if runtime.GOOS == "windows" {
		win := []struct{ name, path string }{
			{"amneziawg", `C:\Program Files\AmneziaWG\amneziawg.exe`},
			{"amneziavpn", `C:\Program Files\AmneziaVPN\AmneziaVPN.exe`},
			{"wireguard", `C:\Program Files\WireGuard\wireguard.exe`},
			{"wireguard", `C:\Program Files\WireGuard\wg.exe`},
		}
		for _, c := range win {
			if st, err := os.Stat(c.path); err == nil && !st.IsDir() {
				return c.name, c.path
			}
		}
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c.name); err == nil {
			return c.name, p
		}
	}
	return "", ""
}


// DetectTunnelDriverClass returns name, path, class (kernel|gui|none).
func DetectTunnelDriverClass() (name, path, class string) {
	name, path = DetectTunnelDriver()
	if path == "" {
		return "", "", "none"
	}
	low := strings.ToLower(name + " " + path)
	if strings.Contains(low, "amneziavpn") {
		return name, path, "gui"
	}
	if strings.Contains(low, "wireguard") || strings.Contains(low, "awg-quick") || strings.Contains(low, "wg-quick") || low == "awg" || strings.HasSuffix(low, "\\awg.exe") || strings.HasSuffix(low, "/awg") {
		return name, path, "kernel"
	}
	if strings.Contains(low, "amneziawg") {
		return name, path, "kernel"
	}
	return name, path, "gui"
}

// GetTunnelStatus snapshot.
func GetTunnelStatus() TunnelStatus {
	tunnel.mu.RLock()
	defer tunnel.mu.RUnlock()
	st := tunnel.status
	if st.Driver == "" {
		n, p := DetectTunnelDriver()
		if n != "" {
			st.Driver = n + ":" + p
			if st.State == "stopped" || st.State == "driver_missing" {
				// don't mutate global here beyond copy
			}
		}
	}
	return st
}

// StartAmneziaTunnel generates keys, writes conf, tries kernel apply.
func StartAmneziaTunnel(req TunnelStartRequest) (TunnelStatus, error) {
	if err := ValidateAmneziaEndpoint(req.Endpoint); err != nil {
		return GetTunnelStatus(), err
	}
	if strings.TrimSpace(req.PeerPublicKey) == "" {
		return GetTunnelStatus(), fmt.Errorf("peerPublicKey required")
	}
	if req.Address == "" {
		req.Address = "10.66.66.2/32"
	}
	if req.AllowedIPs == "" {
		req.AllowedIPs = "0.0.0.0/0, ::/0"
	}

	cfg := GetAmneziaConfig()
	cfg.Enabled = true
	cfg.Endpoint = req.Endpoint
	_ = SetAmneziaConfig(cfg)

	privB64 := req.PrivateKey
	var pubB64 string
	var err error
	if privB64 == "" {
		privB64, pubB64, err = GenerateWGKeyPair()
		if err != nil {
			return GetTunnelStatus(), err
		}
	} else {
		// derive pub if possible
		pubB64, err = PublicKeyFromPrivateB64(privB64)
		if err != nil {
			pubB64 = ""
		}
	}

	conf := BuildAmneziaWGConfFull(privB64, req.PeerPublicKey, req.Endpoint, req.AllowedIPs, req.Address, req.PeerEndpointPresharedKey, cfg)
	confPath := filepath.Join(amneziaDir(), "indestructible.conf")
	if err := os.WriteFile(confPath, []byte(conf), 0o600); err != nil {
		return GetTunnelStatus(), fmt.Errorf("write conf: %w", err)
	}

	// also write public peerless metadata
	meta := fmt.Sprintf("localPublicKey=%s\npeerPublicKey=%s\nendpoint=%s\nupdated=%s\n",
		pubB64, req.PeerPublicKey, req.Endpoint, time.Now().UTC().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(amneziaDir(), "tunnel.meta"), []byte(meta), 0o600)

	driverName, driverPath, driverClass := DetectTunnelDriverClass()

	tunnel.mu.Lock()
	defer tunnel.mu.Unlock()
	// stop previous userspace if any
	if tunnel.userspace != nil {
		_ = tunnel.userspace.Stop()
		tunnel.userspace = nil
	}
	tunnel.privateKeyB64 = privB64
	tunnel.confPath = confPath
	tunnel.status = TunnelStatus{
		State:     "starting",
		ConfPath:  confPath,
		LocalPub:  pubB64,
		PeerPub:   req.PeerPublicKey,
		Endpoint:  req.Endpoint,
		Address:   req.Address,
		Interface: "indestructible",
		Jc:        cfg.JunkPacketCount,
		StartedAt: time.Now().Unix(),
	}
	if driverPath != "" {
		tunnel.status.Driver = driverName + ":" + driverPath
	}

	// 1) Kernel / awg-quick path
	if driverClass == "kernel" && driverPath != "" {
		if err := applyTunnelDriver(driverName, driverPath, confPath); err != nil {
			tunnel.status.LastError = err.Error()
			// fall through to userspace
		} else {
			tunnel.status.State = "up"
			tunnel.status.Mode = "kernel"
			tunnel.status.Note = "Tunnel applied via " + driverName
			return tunnel.status, nil
		}
	}

	// 2) Userspace WireGuard-compatible transport (always available on this host)
	us, err := NewUserspaceVPN(UserspaceConfig{
		ListenAddr:        ":0",
		StaticPrivateKey:  privB64,
		StaticPublicKey:   pubB64,
	})
	if err != nil {
		// still conf_ready
		tunnel.status.State = "conf_ready"
		tunnel.status.Mode = "none"
		tunnel.status.LastError = err.Error()
		tunnel.status.Note = "Conf written; userspace init failed. GUI Amnezia can import conf."
		return tunnel.status, nil
	}
	if err := us.Start(); err != nil {
		tunnel.status.State = "conf_ready"
		tunnel.status.Mode = "none"
		tunnel.status.LastError = err.Error()
		tunnel.status.Note = "Conf written; userspace start failed"
		return tunnel.status, nil
	}
	// Add peer for handshake path
	_ = us.AddPeer(PeerInfo{
		ID:         "amnezia-peer",
		PublicKey:  req.PeerPublicKey,
		Endpoint:   req.Endpoint,
		AllowedIPs: req.AllowedIPs,
	})
	tunnel.userspace = us
	tunnel.status.State = "up"
	tunnel.status.Mode = "userspace"
	if driverClass == "gui" {
		tunnel.status.Note = fmt.Sprintf("In-app userspace VPN UP. External %s optional for conf import: %s", driverName, confPath)
	} else if driverClass == "kernel" {
		tunnel.status.Note = "Userspace tunnel UP (kernel apply failed earlier). Conf: " + confPath
	} else {
		tunnel.status.Note = "In-app userspace VPN UP (no external app). Optional conf: " + confPath
	}
	return tunnel.status, nil
}

func applyTunnelDriver(name, path, confPath string) error {
	switch {
	case strings.Contains(name, "wireguard") && runtime.GOOS == "windows":
		// wireguard /installtunnelservice conf
		cmd := exec.Command(path, "/installtunnelservice", confPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("wireguard install: %w (%s)", err, truncateStr(string(out), 200))
		}
		return nil
	case name == "awg-quick" || name == "wg-quick":
		cmd := exec.Command(path, "up", confPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s up: %w (%s)", name, err, truncateStr(string(out), 200))
		}
		return nil
	case name == "awg" || name == "wg":
		// conf must be installed manually on some systems
		return fmt.Errorf("%s found but needs awg-quick/wg-quick for conf apply", name)
	default:
		// AmneziaVPN GUI — cannot fully automate; conf is ready for import
		return fmt.Errorf("driver %s is GUI-only; import conf at %s", name, confPath)
	}
}

// StopAmneziaTunnel stops tunnel if driver supports it.
func StopAmneziaTunnel() TunnelStatus {
	tunnel.mu.Lock()
	defer tunnel.mu.Unlock()
	if tunnel.userspace != nil {
		_ = tunnel.userspace.Stop()
		tunnel.userspace = nil
	}
	if tunnel.confPath != "" {
		name, path := DetectTunnelDriver()
		if path != "" && strings.Contains(name, "wireguard") && runtime.GOOS == "windows" {
			_ = exec.Command(path, "/uninstalltunnelservice", "indestructible").Run()
		}
		if path != "" && (name == "awg-quick" || name == "wg-quick") {
			_ = exec.Command(path, "down", tunnel.confPath).Run()
		}
	}
	tunnel.status.State = "stopped"
	tunnel.status.Mode = "none"
	tunnel.status.LastError = ""
	tunnel.status.Note = "stopped"
	tunnel.status.StartedAt = 0
	return tunnel.status
}

// BuildAmneziaWGConfFull includes Address + optional PSK.
func BuildAmneziaWGConfFull(privateKey, peerPublicKey, endpoint, allowedIPs, address, psk string, cfg AmneziaConfig) string {
	if allowedIPs == "" {
		allowedIPs = "0.0.0.0/0, ::/0"
	}
	if address == "" {
		address = "10.66.66.2/32"
	}
	if cfg.JunkPacketCount == 0 {
		cfg = DefaultAmneziaConfig()
		cfg.Enabled = true
	}
	pskLine := ""
	if psk != "" {
		pskLine = "PresharedKey = " + psk + "\n"
	}
	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
Jc = %d
Jmin = %d
Jmax = %d
# AmneziaWG DPI obfuscation (table 02) — Indestructible

[Peer]
PublicKey = %s
%sEndpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`, privateKey, address, cfg.JunkPacketCount, cfg.JunkPacketMinSize, cfg.JunkPacketMaxSize, peerPublicKey, pskLine, endpoint, allowedIPs)
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// RandomHex helper
func RandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
