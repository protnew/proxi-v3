package vpn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
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
	ID         string `json:"id"`
	Name       string `json:"name"`
	PublicKey  string `json:"publicKey"`
	Endpoint   string `json:"endpoint,omitempty"`
	AllowedIPs string `json:"allowedIPs"`
	IsExitNode bool   `json:"isExitNode"`
	Online     bool   `json:"online"`
	BytesSent  int64  `json:"bytesSent"`
	BytesRecv  int64  `json:"bytesRecv"`
	LastSeen   int64  `json:"lastSeen"`
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
	Transport string   `json:"transport"`
}

// Manager — управление WireGuard VPN
type Manager struct {
	mu        sync.RWMutex
	state     VPNState
	config    Config
	peers     map[string]*Peer
	privKey   string
	pubKey    string
	myIP      string
	startTime time.Time
	cancel    context.CancelFunc
	stubMode  bool          // true if wg binary is not available — runs without real interface
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
			ListenAddr:       fmt.Sprintf(":%d", m.config.Port),
			MTU:              m.config.MTU,
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

	transport := "kernel"
	if m.stubMode {
		transport = "stub"
	} else if m.userspace != nil {
		transport = "userspace"
	}

	return Status{
		State:     m.state,
		MyIP:      m.myIP,
		MyPubKey:  m.pubKey,
		Peers:     peers,
		Uptime:    uptime,
		Transport: transport,
	}
}

// AddPeer — добавить друга (ручной exchange)
