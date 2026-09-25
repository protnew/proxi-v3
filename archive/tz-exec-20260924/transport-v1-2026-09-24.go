package vpn

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// TransportStats holds traffic statistics for a UDP transport.
type TransportStats struct {
	BytesSent   int64 `json:"bytesSent"`
	BytesRecv   int64 `json:"bytesRecv"`
	PacketsSent int64 `json:"packetsSent"`
	PacketsRecv int64 `json:"packetsRecv"`
	Errors      int64 `json:"errors"`
}

// UDPTransport wraps a UDP connection for userspace VPN packet transport.
type UDPTransport struct {
	conn      *net.UDPConn
	peerAddr  *net.UDPAddr
	localPort int
	peerPub   string
	sharedKey []byte
	sendCh    chan []byte
	recvCh    chan []byte
	closed    atomic.Bool

	mu          sync.Mutex
	seq         uint64
	bytesSent   atomic.Int64
	bytesRecv   atomic.Int64
	packetsSent atomic.Int64
	packetsRecv atomic.Int64
	errors      atomic.Int64
}

// NewUDPTransport creates a new UDP transport bound to localPort, sending to peerAddr.
// sharedKey must be 32 bytes (AES-256).
func NewUDPTransport(localPort int, peerAddr string, sharedKey []byte) (*UDPTransport, error) {
	if len(sharedKey) != 32 {
		return nil, fmt.Errorf("shared key must be 32 bytes, got %d", len(sharedKey))
	}

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return nil, fmt.Errorf("resolve local addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen UDP: %w", err)
	}

	// Get actual port in case localPort was 0
	actualPort := conn.LocalAddr().(*net.UDPAddr).Port

	var peer *net.UDPAddr
	if peerAddr != "" {
		peer, err = net.ResolveUDPAddr("udp", peerAddr)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("resolve peer addr: %w", err)
		}
	}

	key := make([]byte, 32)
	copy(key, sharedKey)

	t := &UDPTransport{
		conn:      conn,
		peerAddr:  peer,
		localPort: actualPort,
		sharedKey: key,
		sendCh:    make(chan []byte, 256),
		recvCh:    make(chan []byte, 256),
	}

	return t, nil
}

// Send encrypts data and sends it via UDP to the peer.
func (t *UDPTransport) Send(data []byte) error {
	if t.closed.Load() {
		return fmt.Errorf("transport closed")
	}
	if t.peerAddr == nil {
		return fmt.Errorf("no peer address configured")
	}

	t.mu.Lock()
	t.seq++
	seq := t.seq
	t.mu.Unlock()

	packet, err := encryptPacket(t.sharedKey, data, seq)
	if err != nil {
		t.errors.Add(1)
		return fmt.Errorf("encrypt: %w", err)
	}

	n, err := t.conn.WriteToUDP(packet, t.peerAddr)
	if err != nil {
		t.errors.Add(1)
		return fmt.Errorf("write: %w", err)
	}

	t.bytesSent.Add(int64(n))
	t.packetsSent.Add(1)
	return nil
}

// Receive reads one UDP packet, decrypts it, and returns the plaintext.
func (t *UDPTransport) Receive() ([]byte, error) {
	if t.closed.Load() {
		return nil, fmt.Errorf("transport closed")
	}

	buf := make([]byte, 65535)
	n, _, err := t.conn.ReadFromUDP(buf)
	if err != nil {
		t.errors.Add(1)
		return nil, fmt.Errorf("read: %w", err)
	}

	plaintext, _, err := decryptPacket(t.sharedKey, buf[:n])
	if err != nil {
		t.errors.Add(1)
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	t.bytesRecv.Add(int64(n))
	t.packetsRecv.Add(1)
	return plaintext, nil
}

// Close shuts down the UDP transport.
func (t *UDPTransport) Close() error {
	t.closed.Store(true)
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

// LocalAddr returns the local UDP address string.
func (t *UDPTransport) LocalAddr() string {
	if t.conn != nil {
		return t.conn.LocalAddr().String()
	}
	return ""
}

// Stats returns current transport statistics.
func (t *UDPTransport) Stats() TransportStats {
	return TransportStats{
		BytesSent:   t.bytesSent.Load(),
		BytesRecv:   t.bytesRecv.Load(),
		PacketsSent: t.packetsSent.Load(),
		PacketsRecv: t.packetsRecv.Load(),
		Errors:      t.errors.Load(),
	}
}

// encryptPacket does AES-GCM encrypt with sequence number.
// Wire format: [seq:8 bytes big-endian][nonce:12 bytes][ciphertext+tag]
func encryptPacket(key, plaintext []byte, seq uint64) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Associated data is the sequence number
	var seqBuf [8]byte
	binary.BigEndian.PutUint64(seqBuf[:], seq)

	// Encrypt with sequence number as additional data
	ciphertext := aead.Seal(nil, nonce, plaintext, seqBuf[:])

	// Build packet: seq || nonce || ciphertext+tag
	packet := make([]byte, 0, 8+len(nonce)+len(ciphertext))
	packet = append(packet, seqBuf[:]...)
	packet = append(packet, nonce...)
	packet = append(packet, ciphertext...)

	return packet, nil
}

// decryptPacket does AES-GCM decrypt.
// Returns plaintext and the sequence number from the packet.
func decryptPacket(key, ciphertext []byte) ([]byte, uint64, error) {
	if len(ciphertext) < 8+12+16 { // seq + nonce + GCM tag minimum
		return nil, 0, fmt.Errorf("ciphertext too short: %d bytes", len(ciphertext))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, 0, fmt.Errorf("aes new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, 0, fmt.Errorf("new gcm: %w", err)
	}

	// Parse seq
	seq := binary.BigEndian.Uint64(ciphertext[:8])

	// Parse nonce
	nonce := ciphertext[8 : 8+aead.NonceSize()]

	// Parse ciphertext+tag
	ct := ciphertext[8+aead.NonceSize():]

	// Associated data is the sequence number
	var seqBuf [8]byte
	binary.BigEndian.PutUint64(seqBuf[:], seq)

	plaintext, err := aead.Open(nil, nonce, ct, seqBuf[:])
	if err != nil {
		return nil, 0, fmt.Errorf("gcm open: %w", err)
	}

	return plaintext, seq, nil
}
