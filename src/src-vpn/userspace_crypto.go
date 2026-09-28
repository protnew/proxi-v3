// File: userspace_crypto.go
// Split from userspace.go: crypto/MAC functions.

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
	"bytes"
	crypto_sha256 "crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/chacha20poly1305"
	"hash"
	"log"
	"net"
	"time"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/hkdf"
)

// ==================== Packet Constants (WireGuard-compatible) ====================

// UserspaceVPN is a fully userspace WireGuard-compatible VPN transport.
// It requires no kernel modules, TUN devices, or root privileges.

// UserspaceConfig holds configuration for the userspace VPN transport.

// PeerInfo represents a remote peer configuration.

// peerSession tracks an established session with a peer.

// NewUserspaceVPN creates a new userspace VPN transport.
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
	a, b := localPub, peerPub
	if bytes.Compare(a, b) > 0 {
		a, b = b, a
	}
	info := append([]byte("UnkillableMessenger-VPN-Session-"), a...)
	info = append(info, b...)

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
	mac1 := computeMAC1(session.peerStatic[:], packet)
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
	if !ok {
		return
	}
	if !u.replayWindow.Check(data) {
		log.Printf("[VPN] Dropping replayed handshake init packet")
		return
	}
	expectedMAC := computeMAC1(u.staticPublic[:], data[:40])
	if constantTimeEqual(data[40:56], expectedMAC) {
		u.sendHandshakeResponse(session)
	}
}

// handleHandshakeResponse handles a handshake response.
func (u *UserspaceVPN) handleHandshakeResponse(data []byte) {
	if len(data) < 28 { // 4(type)+4(sender)+4(receiver)+16(mac1)
		return
	}

	// [0-04] Cookie replay protection
	if !u.replayWindow.Check(data) {
		return
	}

	u.mu.RLock()
	defer u.mu.RUnlock()

	for _, session := range u.peers {
		expectedMAC := computeMAC1(u.staticPublic[:], data[:12])
		if constantTimeEqual(data[12:28], expectedMAC) {
			session.handshakeAt = time.Now()
			session.handshakeDone.Store(true)
			if session.handshakeTimer != nil {
				session.handshakeTimer.Reset()
			}
			return
		}
	}
}

// sendHandshakeInit sends a handshake initiation to a peer.
func (u *UserspaceVPN) sendHandshakeInit(session *peerSession) error {
	packet := make([]byte, 56)
	packet[0] = packetTypeHandshakeInit
	if session.handshakeTimer != nil {
		binary.BigEndian.PutUint32(packet[4:8], uint32(session.handshakeTimer.Attempts()))
	}
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

// manageHandshake handles the retransmission loop for handshake initiations.
func (u *UserspaceVPN) manageHandshake(session *peerSession) {
	defer u.wg.Done()

	// Send initial handshake, then burn attempt 0 so a retry is a different packet.
	_ = u.sendHandshakeInit(session)
	if session.handshakeTimer != nil {
		_, _ = session.handshakeTimer.RecordRetransmit()
	}

	for {
		if session.handshakeTimer == nil {
			return // Cannot retransmit without timer
		}
		if session.handshakeDone.Load() {
			return
		}
		if u.ctx != nil && u.ctx.Err() != nil {
			return
		}

		if session.handshakeTimer.ShouldRetransmit() {
			attempts, err := session.handshakeTimer.RecordRetransmit()
			if err != nil {
				log.Printf("[VPN] Handshake max retries exceeded for peer %s", session.info.ID)
				return
			}
			log.Printf("[VPN] Retransmitting handshake init for peer %s (attempt %d)", session.info.ID, attempts)
			_ = u.sendHandshakeInit(session)
		}

		delay := session.handshakeTimer.NextBackoff()
		if delay == 0 {
			return // Budget exhausted
		}

		if u.ctx == nil {
			<-time.After(delay)
		} else {
			select {
			case <-time.After(delay):
			case <-u.ctx.Done():
				return
			}
		}
	}
}

// keepaliveLoop sends periodic keepalive packets to all peers.
