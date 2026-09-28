package vpn

import (
	"crypto/rand"
	"fmt"
	"net"
	"testing"
)

func TestUDPTransport_CreateAndClose(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tr, err := NewUDPTransport(0, "127.0.0.1:12345", key)
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	defer tr.Close()

	addr := tr.LocalAddr()
	if addr == "" {
		t.Error("LocalAddr should not be empty")
	}
	t.Logf("LocalAddr: %s", addr)

	if tr.closed.Load() {
		t.Error("should not be closed initially")
	}

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	if !tr.closed.Load() {
		t.Error("should be closed after Close()")
	}
}

func TestEncryptDecryptPacket(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	plaintext := []byte("Hello, VPN transport!")
	encrypted, err := encryptPacket(key, plaintext, 1)
	if err != nil {
		t.Fatalf("encryptPacket: %v", err)
	}

	decrypted, seq, err := decryptPacket(key, encrypted)
	if err != nil {
		t.Fatalf("decryptPacket: %v", err)
	}

	if seq != 1 {
		t.Errorf("seq = %d, want 1", seq)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecryptPacket_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatalf("generate key1: %v", err)
	}
	if _, err := rand.Read(key2); err != nil {
		t.Fatalf("generate key2: %v", err)
	}

	plaintext := []byte("secret message")
	encrypted, err := encryptPacket(key1, plaintext, 1)
	if err != nil {
		t.Fatalf("encryptPacket: %v", err)
	}

	_, _, err = decryptPacket(key2, encrypted)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestEncryptDecryptPacket_Tampered(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	plaintext := []byte("important data")
	encrypted, err := encryptPacket(key, plaintext, 1)
	if err != nil {
		t.Fatalf("encryptPacket: %v", err)
	}

	// Tamper with the ciphertext (flip a bit in the encrypted payload)
	if len(encrypted) > 25 {
		encrypted[24] ^= 0xFF
	}

	_, _, err = decryptPacket(key, encrypted)
	if err == nil {
		t.Error("expected error when decrypting tampered packet")
	}
}

func TestTransportStats(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tr1, err := NewUDPTransport(0, "", key)
	if err != nil {
		t.Fatalf("NewUDPTransport tr1: %v", err)
	}
	defer tr1.Close()

	tr2, err := NewUDPTransport(0, tr1.LocalAddr(), key)
	if err != nil {
		t.Fatalf("NewUDPTransport tr2: %v", err)
	}
	defer tr2.Close()

	// Set up peer addresses for bidirectional communication using localhost
	tr1.peerAddr, _ = net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", tr2.localPort))
	tr2.peerAddr, _ = net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", tr1.localPort))

	// Send 3 packets
	for i := 0; i < 3; i++ {
		if err := tr1.Send([]byte("hello")); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}

	stats := tr1.Stats()
	if stats.PacketsSent != 3 {
		t.Errorf("PacketsSent = %d, want 3", stats.PacketsSent)
	}
	if stats.BytesSent == 0 {
		t.Error("BytesSent should be > 0")
	}
	if stats.Errors != 0 {
		t.Errorf("Errors = %d, want 0", stats.Errors)
	}

	// Receive and check stats on tr2
	for i := 0; i < 3; i++ {
		_, err := tr2.Receive()
		if err != nil {
			t.Fatalf("Receive %d: %v", i, err)
		}
	}

	stats2 := tr2.Stats()
	if stats2.PacketsRecv != 3 {
		t.Errorf("PacketsRecv = %d, want 3", stats2.PacketsRecv)
	}
	if stats2.BytesRecv == 0 {
		t.Error("BytesRecv should be > 0")
	}
}
