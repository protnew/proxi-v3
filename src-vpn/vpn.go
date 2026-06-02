package vpn

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net"
    "os"
    "os/exec"
    "path/filepath"
    "sync"
    "time"

    "golang.org/x/crypto/curve25519"
)

// VPNState — состояние VPN
type VPNState string

const (
    StateDisconnected VPNState = "disconnected"
    StateConnecting   VPNState = "connecting"
    StateConnected    VPNState = "connected"
    StateSharing      VPNState = "sharing"
    StateError        VPNState = "error"
)

// Peer — узел в mesh-сети
type Peer struct {
    ID           string `json:"id"`
    Name         string `json:"name"`
    PublicKey    string `json:"publicKey"`
    Endpoint     string `json:"endpoint,omitempty"`
    AllowedIPs   string `json:"allowedIPs"`
    IsExitNode   bool   `json:"isExitNode"`
    Online       bool   `json:"online"`
    BytesSent    int64  `json:"bytesSent"`
    BytesRecv    int64  `json:"bytesRecv"`
    LastSeen     int64  `json:"lastSeen"`
}

// Config — конфигурация VPN
type Config struct {
    InterfaceName string `json:"interfaceName"`
    Address       string `json:"address"`
    Port          int    `json:"port"`
    MTU           int    `json:"mtu"`
    DataDir       string `json:"dataDir"`
}

// Status — полный статус для UI
type Status struct {
    State     VPNState `json:"state"`
    MyIP      string   `json:"myIP"`
    MyPubKey  string   `json:"myPublicKey"`
    Peers     []Peer   `json:"peers"`
    Uptime    int64    `json:"uptime"`
    BytesUp   int64    `json:"bytesUp"`
    BytesDown int64    `json:"bytesDown"`
}

// Manager — управление WireGuard VPN
type Manager struct {
    mu       sync.RWMutex
    state    VPNState
    config   Config
    peers    map[string]*Peer
    privKey  string
    pubKey   string
    myIP     string
    startTime time.Time
    cancel   context.CancelFunc
    stubMode bool // true if wg binary is not available — runs without real interface
    userspace *UserspaceVPN // userspace transport when kernel WG unavailable
}

// NewManager создаёт VPN менеджер
func NewManager(dataDir string) (*Manager, error) {
    if dataDir == "" {
        home, _ := os.UserHomeDir()
        dataDir = filepath.Join(home, ".unkillable-vpn")
    }
    os.MkdirAll(dataDir, 0700)

    m := &Manager{
        state: StateDisconnected,
        config: Config{
            InterfaceName: "um0",
            Address:       "10.77.0.1/24",
            Port:          51820,
            MTU:           1280,
            DataDir:       dataDir,
        },
        peers: make(map[string]*Peer),
    }

    // Load or generate keys
    if err := m.loadOrGenerateKeys(); err != nil {
        return nil, fmt.Errorf("key init: %w", err)
    }

    // If kernel WireGuard is unavailable, initialize userspace transport
    if !m.IsWireGuardAvailable() {
        fmt.Printf("[VPN] Kernel WireGuard not found — initializing userspace transport\n")
        us, err := NewUserspaceVPN(UserspaceConfig{
            ListenAddr:      fmt.Sprintf(":%d", m.config.Port),
            MTU:             m.config.MTU,
            StaticPrivateKey: m.privKey,
            StaticPublicKey:  m.pubKey,
        })
        if err != nil {
            fmt.Printf("[VPN] Warning: userspace init failed: %v (falling back to stub mode)\n", err)
            m.stubMode = true
        } else {
            m.userspace = us
            m.stubMode = false // userspace transport replaces kernel WG
            fmt.Printf("[VPN] Userspace transport ready (public key: %s)\n", us.GetPublicKey())
        }
    }

    return m, nil
}

// loadOrGenerateKeys загружает или создаёт WireGuard ключи
func (m *Manager) loadOrGenerateKeys() error {
    keyFile := filepath.Join(m.config.DataDir, "private.key")
    
    data, err := os.ReadFile(keyFile)
    if err == nil && len(data) > 0 {
        m.privKey = string(data)
        // Derive public key from private key
        m.pubKey, err = m.derivePublicKey(m.privKey)
        if err != nil {
            return err
        }
        return nil
    }

    // Generate new keys
    m.privKey, m.pubKey, err = m.generateKeyPair()
    if err != nil {
        return err
    }
    
    return os.WriteFile(keyFile, []byte(m.privKey), 0600)
}

// generateKeyPair — генерация WireGuard ключевой пары (curve25519, без wg CLI)
func (m *Manager) generateKeyPair() (private, public string, err error) {
    var privateKey [32]byte
    if _, err = rand.Read(privateKey[:]); err != nil {
        return "", "", err
    }
    // WireGuard: clamp private key
    privateKey[0] &= 248
    privateKey[31] = (privateKey[31] & 127) | 64

    var publicKey [32]byte
    curve25519.ScalarBaseMult(&publicKey, &privateKey)

    private = base64.StdEncoding.EncodeToString(privateKey[:])
    public = base64.StdEncoding.EncodeToString(publicKey[:])
    return
}

// derivePublicKey — получение публичного ключа из приватного (curve25519)
func (m *Manager) derivePublicKey(privKey string) (string, error) {
    keyBytes, err := base64.StdEncoding.DecodeString(privKey)
    if err != nil {
        // try hex
        keyBytes, err = hex.DecodeString(privKey)
        if err != nil {
            return privKey, nil // fallback
        }
    }
    var privateKey [32]byte
    copy(privateKey[:], keyBytes)
    var publicKey [32]byte
    curve25519.ScalarBaseMult(&publicKey, &privateKey)
    return base64.StdEncoding.EncodeToString(publicKey[:]), nil
}

// StartExitNode — начать делиться интернетом (стать exit node)
func (m *Manager) StartExitNode(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.state == StateSharing {
        return fmt.Errorf("already sharing")
    }

    m.state = StateConnecting

    // Check if WireGuard is available; fall back to userspace or stub mode
    if m.userspace != nil {
        // Use userspace transport — no kernel interface needed
        if err := m.userspace.Start(); err != nil {
            m.state = StateError
            return fmt.Errorf("userspace start: %w", err)
        }
        m.myIP = "10.77.0.1"
        fmt.Printf("[VPN] Userspace transport started\n")
    } else if !m.IsWireGuardAvailable() {
        fmt.Printf("[VPN] WARNING: wg binary not found — switching to stub mode (no real VPN interface)\n")
        m.stubMode = true
    }

    // Создаём WireGuard интерфейс (only if not userspace)
    if m.userspace == nil {
        if err := m.setupInterface(); err != nil {
            m.state = StateError
            return fmt.Errorf("setup interface: %w", err)
        }
    }

    // Включаем IP forwarding (для exit node)
    if err := m.enableIPForwarding(); err != nil {
        // Не критично — может не быть прав
        fmt.Printf("Warning: IP forwarding: %v\n", err)
    }

    // Настраиваем NAT (iptables/nftables)
    if err := m.setupNAT(); err != nil {
        fmt.Printf("Warning: NAT setup: %v\n", err)
    }

    ctx, cancel := context.WithCancel(ctx)
    m.cancel = cancel
    m.startTime = time.Now()
    m.state = StateSharing

    // Запускаем heartbeat — проверяем пиров
    go m.heartbeatLoop(ctx)

    return nil
}

// StopExitNode — перестать делиться интернетом
func (m *Manager) StopExitNode() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.cancel != nil {
        m.cancel()
    }

    // Stop userspace transport if active
    if m.userspace != nil {
        m.userspace.Stop()
    }

    if err := m.teardownInterface(); err != nil {
        fmt.Printf("Warning: teardown: %v\n", err)
    }

    m.state = StateDisconnected
    return nil
}

// ConnectToExitNode — подключиться к exit node друга
func (m *Manager) ConnectToExitNode(ctx context.Context, peerPubKey, endpoint string) error {
    if len(peerPubKey) < 16 {
        return fmt.Errorf("public key too short (min 16 chars, got %d)", len(peerPubKey))
    }
    m.mu.Lock()
    defer m.mu.Unlock()

    m.state = StateConnecting

    // Check if WireGuard is available; fall back to userspace or stub mode
    if m.userspace != nil {
        // Use userspace transport
        if err := m.userspace.Start(); err != nil {
            m.state = StateError
            return fmt.Errorf("userspace start: %w", err)
        }
        m.myIP = "10.77.0.1"
        fmt.Printf("[VPN] Userspace transport started for peer connection\n")
    } else if !m.IsWireGuardAvailable() {
        fmt.Printf("[VPN] WARNING: wg binary not found — switching to stub mode (no real VPN interface)\n")
        m.stubMode = true
    }

    // Добавляем пир
    peer := &Peer{
        ID:         peerPubKey[:16],
        PublicKey:  peerPubKey,
        Endpoint:   endpoint,
        AllowedIPs: "0.0.0.0/0", // весь трафик через exit node
        IsExitNode: true,
        Online:     false,
    }
    m.peers[peer.ID] = peer

    // Add peer to userspace transport if available
    if m.userspace != nil {
        if err := m.userspace.AddPeer(PeerInfo{
            ID:        peer.ID,
            PublicKey: peer.PublicKey,
            Endpoint:  peer.Endpoint,
        }); err != nil {
            fmt.Printf("[VPN] Warning: userspace add peer: %v\n", err)
        }
    }

    // Создаём интерфейс (only if not userspace)
    if m.userspace == nil {
        if err := m.setupInterface(); err != nil {
            m.state = StateError
            return err
        }
    }

    // Добавляем пира
    if err := m.addPeer(peer); err != nil {
        m.state = StateError
        return err
    }

    // Меняем маршрут — весь трафик через exit node
    if err := m.routeAllTraffic(endpoint); err != nil {
        fmt.Printf("Warning: route all: %v\n", err)
    }

    m.state = StateConnected
    return nil
}

// Disconnect — отключиться
func (m *Manager) Disconnect() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if m.cancel != nil {
        m.cancel()
    }
    if m.userspace != nil {
        m.userspace.Stop()
    }
    m.teardownInterface()
    m.state = StateDisconnected
    return nil
}

// GetStatus — текущий статус для UI
func (m *Manager) GetStatus() Status {
    m.mu.RLock()
    defer m.mu.RUnlock()

    peers := make([]Peer, 0, len(m.peers))
    for _, p := range m.peers {
        peers = append(peers, *p)
    }

    uptime := int64(0)
    if !m.startTime.IsZero() {
        uptime = int64(time.Since(m.startTime).Seconds())
    }

    return Status{
        State:     m.state,
        MyIP:      m.myIP,
        MyPubKey:  m.pubKey,
        Peers:     peers,
        Uptime:    uptime,
    }
}

// AddPeer — добавить друга (ручной exchange)
func (m *Manager) AddPeer(name, pubKey, endpoint string) error {
    if len(pubKey) < 16 {
        return fmt.Errorf("public key too short (min 16 chars, got %d)", len(pubKey))
    }
    m.mu.Lock()
    defer m.mu.Unlock()

    ip := m.allocatePeerIP()
    peer := &Peer{
        ID:         pubKey[:16],
        Name:       name,
        PublicKey:  pubKey,
        Endpoint:   endpoint,
        AllowedIPs: ip + "/32",
        IsExitNode: false,
        Online:     false,
    }
    m.peers[peer.ID] = peer
    return nil
}

// RemovePeer — удалить пира
func (m *Manager) RemovePeer(peerID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if p, ok := m.peers[peerID]; ok {
        m.removePeerFromInterface(p)
        delete(m.peers, peerID)
    }
    return nil
}

// GetPublicKey — вернуть свой публичный ключ (для обмена)
func (m *Manager) GetPublicKey() string {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.pubKey
}

// ========== Internal methods ==========

func (m *Manager) allocatePeerIP() string {
    i := len(m.peers) + 2 // .1 = мы
    return fmt.Sprintf("10.77.0.%d", i)
}

func (m *Manager) heartbeatLoop(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.checkPeers()
        }
    }
}

func (m *Manager) checkPeers() {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for id, p := range m.peers {
        if p.Endpoint != "" {
            // Ping peer endpoint
            conn, err := net.DialTimeout("udp", p.Endpoint, 3*time.Second)
            if err == nil {
                conn.Close()
                p.Online = true
                p.LastSeen = time.Now().Unix()
            } else {
                p.Online = false
            }
            m.peers[id] = p
        }
    }
}

// ========== Platform-specific (Linux/WSL stubs) ==========

// IsWireGuardAvailable checks whether the wg binary exists on the system.
func (m *Manager) IsWireGuardAvailable() bool {
    _, err := exec.LookPath("wg")
    return err == nil
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
    cmd := exec.Command("ip", "link", "add", "dev", iface, "type", "wireguard")
    if err := cmd.Run(); err != nil {
        // May already exist — check with a second attempt
        fmt.Printf("[VPN] Warning: ip link add: %v (may already exist)\n", err)
    }

    // wg set um0 private-key <key> listen-port 51820
    keyFile := filepath.Join(m.config.DataDir, "private.key")
    if err := exec.Command("wg", "set", iface, "private-key", keyFile, "listen-port", fmt.Sprintf("%d", m.config.Port)).Run(); err != nil {
        return fmt.Errorf("wg set private-key: %w (is wireguard-tools installed?)", err)
    }

    // ip address add 10.77.0.1/24 dev um0
    if err := exec.Command("ip", "address", "add", ip, "dev", iface).Run(); err != nil {
        fmt.Printf("[VPN] Warning: ip address add: %v (may already be assigned)\n", err)
    }

    // ip link set um0 up
    if err := exec.Command("ip", "link", "set", iface, "up").Run(); err != nil {
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
    return exec.Command("ip", "link", "del", m.config.InterfaceName).Run()
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
    if err := exec.Command("wg", args...).Run(); err != nil {
        return fmt.Errorf("wg set peer %s: %w", p.PublicKey[:min(16, len(p.PublicKey))], err)
    }
    return nil
}

func (m *Manager) removePeerFromInterface(p *Peer) error {
    if m.stubMode {
        return nil
    }
    return exec.Command("wg", "set", m.config.InterfaceName, "peer", p.PublicKey, "remove").Run()
}

func (m *Manager) enableIPForwarding() error {
    return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
}

func (m *Manager) setupNAT() error {
    // iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
    return exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-j", "MASQUERADE").Run()
}

func (m *Manager) routeAllTraffic(endpoint string) error {
    // ip route add 0.0.0.0/0 dev um0
    exec.Command("ip", "route", "add", "0.0.0.0/0", "dev", m.config.InterfaceName).Run()
    return nil
}

// ========== JSON-RPC interface for Rust IPC ==========

// HandleRPC — обработка JSON-RPC запроса от Tauri/Rust
func (m *Manager) HandleRPC(request []byte) []byte {
    var req struct {
        Method string          `json:"method"`
        Params json.RawMessage `json:"params"`
    }
    if err := json.Unmarshal(request, &req); err != nil {
        return rpcError(-32700, "parse error")
    }

    var result interface{}
    var err error

    switch req.Method {
    case "get_status":
        result = m.GetStatus()
    case "get_public_key":
        result = map[string]string{"publicKey": m.GetPublicKey()}
    case "start_exit_node":
        err = m.StartExitNode(context.Background())
        if err == nil {
            result = map[string]string{"status": "sharing"}
        }
    case "stop_exit_node":
        err = m.StopExitNode()
        if err == nil {
            result = map[string]string{"status": "stopped"}
        }
    case "connect_to_exit_node":
        var params struct {
            PublicKey string `json:"publicKey"`
            Endpoint  string `json:"endpoint"`
        }
        json.Unmarshal(req.Params, &params)
        err = m.ConnectToExitNode(context.Background(), params.PublicKey, params.Endpoint)
        if err == nil {
            result = map[string]string{"status": "connected"}
        }
    case "disconnect":
        err = m.Disconnect()
        if err == nil {
            result = map[string]string{"status": "disconnected"}
        }
    case "add_peer":
        var params struct {
            Name     string `json:"name"`
            PublicKey string `json:"publicKey"`
            Endpoint string `json:"endpoint"`
        }
        json.Unmarshal(req.Params, &params)
        err = m.AddPeer(params.Name, params.PublicKey, params.Endpoint)
        if err == nil {
            result = map[string]string{"status": "added"}
        }
    case "remove_peer":
        var params struct {
            PeerID string `json:"peerId"`
        }
        json.Unmarshal(req.Params, &params)
        err = m.RemovePeer(params.PeerID)
        if err == nil {
            result = map[string]string{"status": "removed"}
        }
    default:
        return rpcError(-32601, "method not found: "+req.Method)
    }

    if err != nil {
        return rpcError(-32000, err.Error())
    }

    resp, _ := json.Marshal(map[string]interface{}{
        "jsonrpc": "2.0",
        "result":  result,
    })
    return resp
}

func rpcError(code int, message string) []byte {
	resp, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"error":   map[string]interface{}{"code": code, "message": message},
	})
	return resp
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
	exec.Command("iptables", "-t", "mangle", "-N", "PROXI_SPLIT").Run()

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
					exec.Command("iptables", "-t", "mangle", "-A", "PROXI_SPLIT",
						"-d", target, "-j", "MARK", "--set-mark", "1").Run()
				} else {
					// Resolve domain and add rule
					if addrs, err := net.LookupHost(target); err == nil {
						for _, addr := range addrs {
							exec.Command("iptables", "-t", "mangle", "-A", "PROXI_SPLIT",
								"-d", addr, "-j", "MARK", "--set-mark", "1").Run()
						}
					}
				}
			}
		} else if cfg.Mode == "exclude" {
			// All traffic through VPN except listed targets
			for _, target := range cfg.Targets {
				if ip := net.ParseIP(target); ip != nil {
					exec.Command("iptables", "-t", "mangle", "-A", "PROXI_SPLIT",
						"-d", target, "-j", "RETURN").Run()
				}
			}
			// Mark everything else (rules above RETURN first, so excluded targets skip this)
			exec.Command("iptables", "-t", "mangle", "-A", "PROXI_SPLIT",
				"-j", "MARK", "--set-mark", "1").Run()
		}
	}

	// Append unconditional RETURN at the end so unmatched traffic is not blocked
	exec.Command("iptables", "-t", "mangle", "-A", "PROXI_SPLIT", "-j", "RETURN").Run()

	// Connect the chain to PREROUTING so it actually gets evaluated
	exec.Command("iptables", "-t", "mangle", "-A", "PREROUTING", "-j", "PROXI_SPLIT").Run()

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
	if err := exec.Command("iptables", "-t", "mangle", "-D", "PREROUTING", "-j", "PROXI_SPLIT").Run(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("delete PREROUTING jump: %w", err)
	}

	// Flush all rules in the PROXI_SPLIT chain
	if err := exec.Command("iptables", "-t", "mangle", "-F", "PROXI_SPLIT").Run(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("flush PROXI_SPLIT: %w", err)
	}

	// Delete the PROXI_SPLIT chain itself
	if err := exec.Command("iptables", "-t", "mangle", "-X", "PROXI_SPLIT").Run(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("delete PROXI_SPLIT chain: %w", err)
	}

	return firstErr
}

// ==================== DNS through VPN ====================

// DNSConfig stores DNS proxy settings.
type DNSConfig struct {
	Enabled   bool   `json:"enabled"`
	Listen    string `json:"listen"`    // e.g. "127.0.0.1:5353"
	Upstream  string `json:"upstream"`  // e.g. "1.1.1.1:53"
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
