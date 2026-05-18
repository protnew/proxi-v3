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

    // Создаём WireGuard интерфейс
    if err := m.setupInterface(); err != nil {
        m.state = StateError
        return fmt.Errorf("setup interface: %w", err)
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

    // Создаём интерфейс
    if err := m.setupInterface(); err != nil {
        m.state = StateError
        return err
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

func (m *Manager) setupInterface() error {
    iface := m.config.InterfaceName
    ip := m.config.Address
    
    // ip link add um0 type wireguard
    cmd := exec.Command("ip", "link", "add", "dev", iface, "type", "wireguard")
    cmd.Run() // может уже существовать
    
    // wg set um0 private-key <key> listen-port 51820
    keyFile := filepath.Join(m.config.DataDir, "private.key")
    exec.Command("wg", "set", iface, "private-key", keyFile, "listen-port", fmt.Sprintf("%d", m.config.Port)).Run()
    
    // ip address add 10.77.0.1/24 dev um0
    exec.Command("ip", "address", "add", ip, "dev", iface).Run()
    
    // ip link set um0 up
    exec.Command("ip", "link", "set", iface, "up").Run()
    
    m.myIP = "10.77.0.1"
    return nil
}

func (m *Manager) teardownInterface() error {
    return exec.Command("ip", "link", "del", m.config.InterfaceName).Run()
}

func (m *Manager) addPeer(p *Peer) error {
    args := []string{"set", m.config.InterfaceName, "peer", p.PublicKey, "allowed-ips", p.AllowedIPs}
    if p.Endpoint != "" {
        args = append(args, "endpoint", p.Endpoint)
    }
    return exec.Command("wg", args...).Run()
}

func (m *Manager) removePeerFromInterface(p *Peer) error {
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
