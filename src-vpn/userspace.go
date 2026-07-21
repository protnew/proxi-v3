// Package vpn provides a WireGuard-compatible userspace VPN transport.
//
// userspace.go implements a fully userspace VPN that works without kernel modules,
// TUN devices, or root privileges. It uses:
//   - ChaCha20-Poly1305 (AEAD) for WireGuard-compatible encryption
//   - X25519 (curve25519) for key exchange
//   - BLAKE2s for MAC computation (WireGuard-compatible)
//   - UDP sockets for transport
//   - HKDF for session key derivation
//
// This is designed for Docker containers and restricted environments where
// wireguard-go or kernel WireGuard is not available.
package vpn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// ==================== Packet Constants (WireGuard-compatible) ====================

const (
	// Packet types matching WireGuard protocol
	packetTypeHandshakeInit     = 1
	packetTypeHandshakeResponse = 2
	packetTypeData              = 4

	// Sizes
	keySize       = 32 // Curve25519 key size
	nonceSize     = 8  // Counter-based nonce
	mac1Size      = 16 // MAC1 size
	mac2Size      = 16 // MAC2 size
	timestampSize = 12 // Tai64n timestamp
	headerSize    = 4  // Packet type (1) + reserved (3)
	dataOverhead  = headerSize + nonceSize + 4 + chacha20poly1305.Overhead + mac1Size
	maxPacketSize = 1420 // MTU + overhead
	sessionIDSize = 4    // Session identifier

	// Session parameters
	handshakeTimeout  = 5 * time.Second
	sessionDuration   = 2 * time.Minute // Re-key interval (like WireGuard)
	keepaliveInterval = 10 * time.Second
)

// UserspaceVPN is a fully userspace WireGuard-compatible VPN transport.
// It requires no kernel modules, TUN devices, or root privileges.
type UserspaceVPN struct {
	config UserspaceConfig
	state  atomic.Int32 // 0=stopped, 1=running
	conn   *net.UDPConn

	localPort int

	// Keys
	staticPrivate [keySize]byte
	staticPublic  [keySize]byte

	// Peer sessions
	mu    sync.RWMutex
	peers map[string]*peerSession // peerID -> session

	// Stats
	bytesSent   atomic.Int64
	bytesRecv   atomic.Int64
	packetsSent atomic.Int64
	packetsRecv atomic.Int64

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	replayWindow *ReplayWindow
}

// UserspaceConfig holds configuration for the userspace VPN transport.
type UserspaceConfig struct {
	// ListenAddr is the local UDP address to bind (e.g. ":0" for random).
	ListenAddr string
	// MTU for packets (default 1280).
	MTU int
	// StaticPrivateKey is the Curve25519 private key (hex or base64).
	// If empty, a random key is generated.
	StaticPrivateKey string
	// StaticPublicKey is the Curve25519 public key (hex or base64).
	// If empty, it is derived from the private key.
	StaticPublicKey string
	// OnReceive is called when decrypted data is received from a peer.
	OnReceive func(peerID string, plaintext []byte)
}

// PeerInfo represents a remote peer configuration.
type PeerInfo struct {
	ID         string // Unique peer identifier
	PublicKey  string // Peer's Curve25519 public key (hex)
	Endpoint   string // Peer's UDP address (host:port)
	AllowedIPs string // Placeholder for routing (not enforced in userspace)
}

// peerSession tracks an established session with a peer.
type peerSession struct {
	info        PeerInfo
	peerStatic  [keySize]byte
	sessionKey  [keySize]byte
	sendCounter atomic.Uint64
	recvCounter atomic.Uint64
	lastActive  atomic.Int64 // unix timestamp
	handshakeAt time.Time
	conn        *net.UDPConn // cached connection to peer endpoint

	// [0-05] Handshake retransmission
	handshakeTimer *RetransmitTimer
	handshakeDone  atomic.Bool
}

// NewUserspaceVPN creates a new userspace VPN transport.
func NewUserspaceVPN(config UserspaceConfig) (*UserspaceVPN, error) {
	if config.MTU <= 0 {
		config.MTU = 1280
	}
	if config.ListenAddr == "" {
		config.ListenAddr = ":0"
	}

	u := &UserspaceVPN{
		config:       config,
		peers:        make(map[string]*peerSession),
		replayWindow: NewReplayWindow(0), // Uses DefaultReplayWindowSize
	}

	// Initialize keys
	if err := u.initKeys(); err != nil {
		return nil, fmt.Errorf("init keys: %w", err)
	}

	return u, nil
}

// initKeys sets up the static key pair.
func (u *UserspaceVPN) initKeys() error {
	if u.config.StaticPrivateKey != "" {
		keyBytes, err := decodeKey(u.config.StaticPrivateKey)
		if err != nil {
			return fmt.Errorf("decode private key: %w", err)
		}
		copy(u.staticPrivate[:], keyBytes)
	} else {
		// Generate random private key
		if _, err := rand.Read(u.staticPrivate[:]); err != nil {
			return fmt.Errorf("generate private key: %w", err)
		}
		// Clamp for Curve25519
		u.staticPrivate[0] &= 248
		u.staticPrivate[31] = (u.staticPrivate[31] & 127) | 64
	}

	// Derive public key
	pub, err := curve25519.X25519(u.staticPrivate[:], curve25519.Basepoint)
	if err != nil {
		return fmt.Errorf("derive public key: %w", err)
	}
	copy(u.staticPublic[:], pub)

	return nil
}

// GetPublicKey returns the static public key in hex encoding.
func (u *UserspaceVPN) GetPublicKey() string {
	return hex.EncodeToString(u.staticPublic[:])
}

// Start begins listening for UDP packets and starts the keepalive loop.
func (u *UserspaceVPN) Start() error {
	if !u.state.CompareAndSwap(0, 1) {
		return fmt.Errorf("already running")
	}

	addr, err := net.ResolveUDPAddr("udp", u.config.ListenAddr)
	if err != nil {
		u.state.Store(0)
		return fmt.Errorf("resolve listen addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		u.state.Store(0)
		return fmt.Errorf("listen UDP: %w", err)
	}
	u.conn = conn
	if udpAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		u.localPort = udpAddr.Port
	}

	u.ctx, u.cancel = context.WithCancel(context.Background())

	// Read loop
	u.wg.Add(1)
	go u.readLoop()

	// Keepalive loop
	u.wg.Add(1)
	go u.keepaliveLoop()

	return nil
}

// GetLocalPort returns the actual UDP port bound by the listener.
func (u *UserspaceVPN) GetLocalPort() int {
	return u.localPort
}

// Stop shuts down the VPN transport.
func (u *UserspaceVPN) Stop() error {
	if !u.state.CompareAndSwap(1, 0) {
		return nil // already stopped
	}

	if u.cancel != nil {
		u.cancel()
	}
	if u.conn != nil {
		u.conn.Close()
	}
	u.wg.Wait()

	return nil
}

// IsRunning returns whether the transport is active.
func (u *UserspaceVPN) IsRunning() bool {
	return u.state.Load() == 1
}

// GetStats returns runtime statistics.
func (u *UserspaceVPN) GetStats() map[string]interface{} {
	u.mu.RLock()
	peerCount := len(u.peers)
	u.mu.RUnlock()

	return map[string]interface{}{
		"running":     u.IsRunning(),
		"bytesSent":   u.bytesSent.Load(),
		"bytesRecv":   u.bytesRecv.Load(),
		"packetsSent": u.packetsSent.Load(),
		"packetsRecv": u.packetsRecv.Load(),
		"peerCount":   peerCount,
		"publicKey":   hex.EncodeToString(u.staticPublic[:]),
	}
}

// AddPeer adds a remote peer and initiates a handshake.
func (u *UserspaceVPN) AddPeer(info PeerInfo) error {
	peerPubBytes, err := decodeKey(info.PublicKey)
	if err != nil {
		return fmt.Errorf("decode peer public key: %w", err)
	}

	var peerPub [keySize]byte
	copy(peerPub[:], peerPubBytes)

	// Compute shared secret via X25519
	shared, err := curve25519.X25519(u.staticPrivate[:], peerPub[:])
	if err != nil {
		return fmt.Errorf("compute shared secret: %w", err)
	}

	// Derive session key using HKDF-BLAKE2s (WireGuard-style chaining key derivation)
	var sharedSecret [keySize]byte
	copy(sharedSecret[:], shared)

	sessionKey := deriveSessionKey(sharedSecret, u.staticPublic[:], peerPub[:])

	session := &peerSession{
		info:           info,
		peerStatic:     peerPub,
		handshakeAt:    time.Now(),
		handshakeTimer: NewRetransmitTimer(),
	}
	copy(session.sessionKey[:], sessionKey)

	u.mu.Lock()
	u.peers[info.ID] = session
	u.mu.Unlock()

	// Resolve and cache the peer endpoint
	if info.Endpoint != "" {
		raddr, err := net.ResolveUDPAddr("udp", info.Endpoint)
		if err == nil {
			session.conn, _ = net.DialUDP("udp", nil, raddr)
		}
	}

	// [0-05] Initiate handshake with retransmission
	u.wg.Add(1)
	go u.manageHandshake(session)

	return nil
}

// RemovePeer removes a peer by ID.
func (u *UserspaceVPN) RemovePeer(peerID string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if s, ok := u.peers[peerID]; ok {
		if s.conn != nil {
			s.conn.Close()
		}
		delete(u.peers, peerID)
	}
}

// SendToPeer sends encrypted data to a specific peer.
func (u *UserspaceVPN) SendToPeer(peerID string, plaintext []byte) error {
	u.mu.RLock()
	session, ok := u.peers[peerID]
	u.mu.RUnlock()

	if !ok {
		return fmt.Errorf("peer %s not found", peerID)
	}

	if session.conn == nil {
		return fmt.Errorf("peer %s has no endpoint", peerID)
	}

	// Encrypt the payload
	packet, err := u.encryptDataPacket(session, plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	n, err := session.conn.Write(packet)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	u.bytesSent.Add(int64(n))
	u.packetsSent.Add(1)
	session.lastActive.Store(time.Now().Unix())

	return nil
}

// ==================== Encryption / Decryption ====================

// encryptDataPacket creates a WireGuard-style encrypted data packet.
//
// Packet format:
//
//	[type:4][nonce:8][counter:4][ciphertext+tag:~][mac1:16]
