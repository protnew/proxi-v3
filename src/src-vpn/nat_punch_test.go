package vpn

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeUDPConn is a minimal UDPConn implementation for deterministic tests.
type fakeUDPConn struct {
	mu       sync.Mutex
	written  [][]byte
	readResp func() (int, *net.UDPAddr, error)
	closed   bool
}

func (f *fakeUDPConn) WriteToUDP(b []byte, _ *net.UDPAddr) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]byte, len(b))
	copy(cp, b)
	f.written = append(f.written, cp)
	return len(b), nil
}

func (f *fakeUDPConn) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	return f.readResp()
}

func (f *fakeUDPConn) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func TestHolePunch_LifecycleSucceeded(t *testing.T) {
	h := NewHolePunchAttempt(0, "127.0.0.1:9999")
	h.SetProbes(3)
	h.SetTimeout(500 * time.Millisecond)

	remote, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9999")
	conn := &fakeUDPConn{
		// The first ReadFrom simulates a probe reply arriving from the peer.
		readResp: func() (int, *net.UDPAddr, error) {
			return 5, remote, nil
		},
	}
	h.Dialer = func(int) (UDPConn, error) { return conn, nil }

	if h.State() != HolePunchIdle {
		t.Fatalf("initial state = %s, want idle", h.State())
	}
	if err := h.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.State() != HolePunchSucceeded {
		t.Errorf("state = %s, want succeeded", h.State())
	}
	sent := h.SentPackets()
	if len(sent) != 3 {
		t.Errorf("sent %d probes, want 3", len(sent))
	}
}

func TestHolePunch_LifecycleFailed(t *testing.T) {
	h := NewHolePunchAttempt(0, "127.0.0.1:9999")
	h.SetProbes(3)
	h.SetTimeout(200 * time.Millisecond)

	conn := &fakeUDPConn{
		// ReadFrom always errors (simulating a peer that never replies).
		readResp: func() (int, *net.UDPAddr, error) {
			return 0, nil, fmt.Errorf("i/o timeout")
		},
	}
	h.Dialer = func(int) (UDPConn, error) { return conn, nil }

	if err := h.Run(); err == nil {
		t.Fatal("Run should fail when peer does not reply")
	}
	if h.State() != HolePunchFailed {
		t.Errorf("state = %s, want failed", h.State())
	}
	if len(h.SentPackets()) != 3 {
		t.Errorf("sent %d probes, want 3", len(h.SentPackets()))
	}
}

func TestHolePunch_DoubleRunRejected(t *testing.T) {
	h := NewHolePunchAttempt(0, "127.0.0.1:9999")
	h.SetProbes(1)
	h.SetTimeout(100 * time.Millisecond)
	conn := &fakeUDPConn{
		readResp: func() (int, *net.UDPAddr, error) {
			return 0, nil, fmt.Errorf("timeout")
		},
	}
	h.Dialer = func(int) (UDPConn, error) { return conn, nil }

	_ = h.Run() // fails
	// Running again on a non-idle attempt must error without re-opening socket.
	if err := h.Run(); err == nil {
		t.Fatal("second Run should error because attempt is not idle")
	}
}

// TestHolePunch_RealLoopback exercises the real network path on loopback: a
// "peer" socket echoes any received probe back to its sender, and the
// hole-punch attempt should succeed once the echo is received.
func TestHolePunch_RealLoopback(t *testing.T) {
	// Spin up a peer that echoes replies.
	peerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve peer: %v", err)
	}
	peer, err := net.ListenUDP("udp", peerAddr)
	if err != nil {
		t.Fatalf("listen peer: %v", err)
	}
	defer peer.Close()

	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		buf := make([]byte, 64)
		for {
			peer.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, from, err := peer.ReadFromUDP(buf)
			if err != nil {
				return
			}
			// Echo back a confirmation packet to whoever sent the probe.
			if _, err := peer.WriteToUDP(buf[:n], from); err != nil {
				return
			}
		}
	}()

	// Attempt hole punch to the peer's real address. Local port "0" lets the
	// OS choose an ephemeral port for the punch socket.
	if err := AttemptHolePunch("0", peer.LocalAddr().String()); err != nil {
		t.Fatalf("AttemptHolePunch: %v", err)
	}
	// If we got here without error, the punch succeeded.
}
