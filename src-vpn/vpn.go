package vpn

import (
    "context"
    "fmt"
    "sync"
)

// VPNState — текущее состояние VPN
type VPNState int

const (
    Disconnected VPNState = iota
    Connecting
    Connected
    Sharing // acting as exit node
)

func (s VPNState) String() string {
    switch s {
    case Disconnected:
        return "disconnected"
    case Connecting:
        return "connecting"
    case Connected:
        return "connected"
    case Sharing:
        return "sharing"
    default:
        return "unknown"
    }
}

// Peer — друг в mesh-сети
type Peer struct {
    ID         string `json:"id"`
    Name       string `json:"name"`
    IP         string `json:"ip"`
    PublicKey  string `json:"publicKey"`
    IsExitNode bool   `json:"isExitNode"`
    Online     bool   `json:"online"`
}

// Manager — управление VPN
type Manager struct {
    mu     sync.RWMutex
    state  VPNState
    peers  []Peer
    myIP   string
}

// NewManager — создаёт новый VPN менеджер
func NewManager() *Manager {
    return &Manager{
        state: Disconnected,
        peers: []Peer{},
    }
}

// StartExitNode — начать делиться интернетом (exit node)
func (m *Manager) StartExitNode(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.state == Sharing {
        return fmt.Errorf("already sharing")
    }

    m.state = Sharing
    // TODO: интеграция с Netbird — advertise exit node
    // netbird.Up("--advertise-exit-node")
    return nil
}

// StopExitNode — перестать делиться
func (m *Manager) StopExitNode() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.state = Disconnected
    return nil
}

// ConnectToPeer — подключиться к exit node друга
func (m *Manager) ConnectToPeer(ctx context.Context, peerID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.state = Connecting
    // TODO: Netbird — connect to peer's exit node
    m.state = Connected
    return nil
}

// Disconnect — отключиться от VPN
func (m *Manager) Disconnect() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.state = Disconnected
}

// GetStatus — текущее состояние
func (m *Manager) GetStatus() (VPNState, []Peer) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.state, m.peers
}
