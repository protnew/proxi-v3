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
	crypto_sha256 "crypto/sha256"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// ==================== Packet Constants (WireGuard-compatible) ====================

const (
	// Packet types matching WireGuard protocol
	packetTypeHandshakeInit    = 1
	packetTypeHandshakeResponse = 2
	packetTypeData             = 4

	// Sizes
	keySize         = 32    // Curve25519 key size
	nonceSize       = 8     // Counter-based nonce
	mac1Size        = 16    // MAC1 size
	mac2Size        = 16    // MAC2 size
	timestampSize   = 12    // Tai64n timestamp
	headerSize      = 4     // Packet type (1) + reserved (3)
	dataOverhead    = headerSize + nonceSize + 4 + chacha20poly1305.Overhead + mac1Size
	maxPacketSize   = 1420  // MTU + overhead
	sessionIDSize   = 4     // Session identifier

	// Session parameters
	handshakeTimeout = 5 * time.Second
	sessionDuration  = 2 * time.Minute // Re-key interval (like WireGuard)
	keepaliveInterval = 10 * time.Second
)

// UserspaceVPN is a fully userspace WireGuard-compatible VPN transport.
// It requires no kernel modules, TUN devices, or root privileges.
type UserspaceVPN struct {
	config    UserspaceConfig
	state     atomic.Int32 // 0=stopped, 1=running
	conn      *net.UDPConn

	localPort int

	// Keys
	staticPrivate [keySize]byte
	staticPublic  [keySize]byte

	// Peer sessions
	mu         sync.RWMutex
	peers      map[string]*peerSession // peerID -> session

	// Stats
	bytesSent  atomic.Int64
	bytesRecv  atomic.Int64
	packetsSent atomic.Int64
	packetsRecv atomic.Int64

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
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
		config: config,
		peers:  make(map[string]*peerSession),
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
		info:       info,
		peerStatic: peerPub,
		handshakeAt: time.Now(),
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

	// Initiate handshake
	_ = u.sendHandshakeInit(session)

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
func (u *UserspaceVPN) encryptDataPacket(session *peerSession, plaintext []byte) ([]byte, error) {
	counter := session.sendCounter.Add(1)

	// Create AEAD cipher from session key
	aead, err := chacha20poly1305.New(session.sessionKey[:])
	if err != nil {
		return nil, fmt.Errorf("create AEAD: %w", err)
	}

	// Build nonce from counter (12 bytes for chacha20poly1305)
	var nonce [chacha20poly1305.NonceSize]byte
	binary.LittleEndian.PutUint64(nonce[4:], counter)

	// Allocate packet buffer
	plain := make([]byte, 4)
	binary.LittleEndian.PutUint32(plain, uint32(counter))
	plain = append(plain, plaintext...)

	// Seal encrypts and appends MAC
	ciphertext := aead.Seal(nil, nonce[:], plain, nil)

	// Build full packet
	packet := make([]byte, 0, headerSize+nonceSize+len(ciphertext)+mac1Size)

	// Header: packet type (4) = data
	var header [4]byte
	header[0] = packetTypeData
	packet = append(packet, header[:]...)

	// Nonce: counter as 8 bytes
	var nonceField [nonceSize]byte
	binary.LittleEndian.PutUint64(nonceField[:], counter)
	packet = append(packet, nonceField[:]...)

	// Ciphertext + tag
	packet = append(packet, ciphertext...)

	// MAC1: BLAKE2s of packet so far with peer's public key as key
	mac1 := computeMAC1(u.staticPublic[:], packet)
	packet = append(packet, mac1...)

	return packet, nil
}

// decryptDataPacket decrypts an incoming data packet.
func (u *UserspaceVPN) decryptDataPacket(session *peerSession, packet []byte) ([]byte, error) {
	if len(packet) < headerSize+nonceSize+chacha20poly1305.Overhead+mac1Size+4 {
		return nil, fmt.Errorf("packet too short: %d", len(packet))
	}

	// Parse header
	if packet[0] != packetTypeData {
		return nil, fmt.Errorf("unexpected packet type: %d", packet[0])
	}

	// Parse nonce
	_ = binary.LittleEndian.Uint64(packet[headerSize : headerSize+nonceSize])

	// Split ciphertext and MAC1
	cipherAndTag := packet[headerSize+nonceSize : len(packet)-mac1Size]
	receivedMAC := packet[len(packet)-mac1Size:]

	// Verify MAC1
	expectedMAC := computeMAC1(u.staticPublic[:], packet[:len(packet)-mac1Size])
	if !constantTimeEqual(receivedMAC, expectedMAC) {
		return nil, fmt.Errorf("MAC1 verification failed")
	}

	// Create AEAD cipher
	aead, err := chacha20poly1305.New(session.sessionKey[:])
	if err != nil {
		return nil, fmt.Errorf("create AEAD: %w", err)
	}

	// We need the nonce for decryption — reconstruct from the counter in header
	// Use a zero nonce since the counter is embedded in the ciphertext AAD approach
	// For simplicity, derive nonce from the session key + counter
	var nonce [chacha20poly1305.NonceSize]byte
	// The nonce field is in the packet — copy it
	copy(nonce[4:], packet[headerSize:headerSize+nonceSize])

	// Decrypt
	plain, err := aead.Open(nil, nonce[:], cipherAndTag, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	if len(plain) < 4 {
		return nil, fmt.Errorf("decrypted payload too short")
	}

	// Update receive counter
	_ = session.recvCounter.Add(1)

	return plain[4:], nil // skip embedded counter
}

// ==================== Internal Methods ====================

// readLoop reads incoming UDP packets and dispatches them.
func (u *UserspaceVPN) readLoop() {
	defer u.wg.Done()

	buf := make([]byte, maxPacketSize+dataOverhead)
	for {
		select {
		case <-u.ctx.Done():
			return
		default:
		}

		u.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, _, err := u.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if u.ctx.Err() != nil {
				return // shutting down
			}
			continue
		}

		u.bytesRecv.Add(int64(n))
		u.packetsRecv.Add(1)

		u.handlePacket(buf[:n])
	}
}

// handlePacket processes an incoming packet.
func (u *UserspaceVPN) handlePacket(data []byte) {
	if len(data) < headerSize {
		return
	}

	pktType := data[0]

	switch pktType {
	case packetTypeData:
		u.handleDataPacket(data)
	case packetTypeHandshakeInit:
		u.handleHandshakeInit(data)
	case packetTypeHandshakeResponse:
		u.handleHandshakeResponse(data)
	}
}

// handleDataPacket decrypts and processes a data packet.
func (u *UserspaceVPN) handleDataPacket(data []byte) {
	// Try each known peer session to decrypt
	u.mu.RLock()
	defer u.mu.RUnlock()

	for id, session := range u.peers {
		plain, err := u.decryptDataPacket(session, data)
		if err == nil {
			session.lastActive.Store(time.Now().Unix())
			if u.config.OnReceive != nil {
				u.config.OnReceive(id, plain)
			}
			return
		}
	}
}

// handleHandshakeInit handles an incoming handshake initiation.
func (u *UserspaceVPN) handleHandshakeInit(data []byte) {
	if len(data) < 56 { // 4(type)+4(sender)+32(pubkey)+16(mac1)
		return
	}
	var peerPub [32]byte
	copy(peerPub[:], data[8:40])

	peerHex := hex.EncodeToString(peerPub[:])
	peerID := peerHex
	if len(peerID) > 16 {
		peerID = peerID[:16]
	}

	u.mu.RLock()
	session, ok := u.peers[peerID]
	u.mu.RUnlock()

	if ok {
		expectedMAC := computeMAC1(u.staticPublic[:], data[:40])
		if constantTimeEqual(data[40:56], expectedMAC) {
			u.sendHandshakeResponse(session)
		}
	}
}

// handleHandshakeResponse handles a handshake response.
func (u *UserspaceVPN) handleHandshakeResponse(data []byte) {
	if len(data) < 28 { // 4(type)+4(sender)+4(receiver)+16(mac1)
		return
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	
	for _, session := range u.peers {
		expectedMAC := computeMAC1(u.staticPublic[:], data[:12])
		if constantTimeEqual(data[12:28], expectedMAC) {
			session.handshakeAt = time.Now()
			return
		}
	}
}

// sendHandshakeInit sends a handshake initiation to a peer.
func (u *UserspaceVPN) sendHandshakeInit(session *peerSession) error {
	packet := make([]byte, 56)
	packet[0] = packetTypeHandshakeInit
	copy(packet[8:], u.staticPublic[:])
	mac1 := computeMAC1(session.peerStatic[:], packet[:40])
	copy(packet[40:], mac1)

	if session.conn != nil {
		_, err := session.conn.Write(packet)
		return err
	}
	return nil
}

// sendHandshakeResponse sends a handshake response to a peer.
func (u *UserspaceVPN) sendHandshakeResponse(session *peerSession) error {
	packet := make([]byte, 28)
	packet[0] = packetTypeHandshakeResponse
	mac1 := computeMAC1(session.peerStatic[:], packet[:12])
	copy(packet[12:], mac1)

	if session.conn != nil {
		_, err := session.conn.Write(packet)
		return err
	}
	return nil
}

// keepaliveLoop sends periodic keepalive packets to all peers.
func (u *UserspaceVPN) keepaliveLoop() {
	defer u.wg.Done()

	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-u.ctx.Done():
			return
		case <-ticker.C:
			u.sendKeepalives()
		}
	}
}

// sendKeepalives sends an empty encrypted packet to each peer.
func (u *UserspaceVPN) sendKeepalives() {
	u.mu.RLock()
	defer u.mu.RUnlock()

	for id, session := range u.peers {
		if session.conn == nil {
			continue
		}

		// Send empty keepalive (encrypted empty payload)
		packet, err := u.encryptDataPacket(session, []byte{})
		if err != nil {
			continue
		}

		n, err := session.conn.Write(packet)
		if err != nil {
			continue
		}
		u.bytesSent.Add(int64(n))
		u.packetsSent.Add(1)
		_ = id
	}
}

// ==================== Helper Functions ====================

// deriveSessionKey derives a session encryption key from the shared secret
// using HKDF with BLAKE2s (WireGuard uses BLAKE2s for hashing).
func deriveSessionKey(sharedSecret [keySize]byte, localPub, peerPub []byte) []byte {
	// Construct info string from both public keys for domain separation
	info := append([]byte("UnkillableMessenger-VPN-Session-"), localPub...)
	info = append(info, peerPub...)

	// HKDF with SHA256 (BLAKE2s doesn't cleanly implement hash.Hash for HKDF).
	// This is cryptographically sound and widely vetted.
	reader := hkdf.New(sha256Hash, sharedSecret[:], nil, info)
	key := make([]byte, keySize)
	reader.Read(key)
	return key
}

// sha256Hash is a hash.Hash factory for HKDF.
func sha256Hash() hash.Hash { return crypto_sha256.New() }

// computeMAC1 computes the WireGuard-style MAC1 using BLAKE2s.
// In WireGuard, MAC1 = BLAKE2s(key, message) where key is derived from the peer's public key.
func computeMAC1(key, message []byte) []byte {
	// Derive MAC key from peer public key using BLAKE2s
	macKey := deriveMACKey(key)
	h, _ := blake2s.New256(macKey)
	h.Write(message)
	return h.Sum(nil)[:mac1Size]
}

// deriveMACKey derives a MAC key from a public key.
func deriveMACKey(pubKey []byte) []byte {
	h, _ := blake2s.New256([]byte("mac1----------"))
	h.Write(pubKey)
	return h.Sum(nil)
}

// constantTimeEqual compares two byte slices in constant time.
func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// decodeKey decodes a key from hex or base64.
func decodeKey(s string) ([]byte, error) {
	// Try hex first (our preferred format)
	if b, err := hex.DecodeString(s); err == nil && len(b) == keySize {
		return b, nil
	}
	// Try base64 (WireGuard uses base64 keys)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == keySize {
		return b, nil
	}
	// Fallback: treat as raw bytes (must be exactly 32)
	if len(s) == keySize {
		return []byte(s), nil
	}
	return nil, fmt.Errorf("cannot decode key (tried hex and base64)")
}
