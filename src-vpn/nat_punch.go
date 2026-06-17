package vpn

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// HolePunchState describes the lifecycle of a hole-punch attempt.
type HolePunchState int

const (
	// HolePunchIdle means the attempt has been created but not started.
	HolePunchIdle HolePunchState = iota
	// HolePunchPunching means probe packets are being sent.
	HolePunchPunching
	// HolePunchSucceeded means a probe from the remote peer was received.
	HolePunchSucceeded
	// HolePunchFailed means the attempt exhausted its probes without success.
	HolePunchFailed
)

func (s HolePunchState) String() string {
	switch s {
	case HolePunchIdle:
		return "idle"
	case HolePunchPunching:
		return "punching"
	case HolePunchSucceeded:
		return "succeeded"
	case HolePunchFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// DefaultHolePunchProbes is the number of probe datagrams sent per attempt.
const DefaultHolePunchProbes = 8

// DefaultHolePunchTimeout is the maximum time an attempt runs before failing.
var DefaultHolePunchTimeout = 2 * time.Second

// HolePunchAttempt coordinates a UDP hole-punching exchange against a single
// remote peer. It opens a local socket, sends a short burst of probe packets
// to the remote address, and listens for any inbound datagram from the peer to
// confirm the NAT pinhole is open.
//
// A Dialer field is exposed so tests can substitute a fake UDP socket without
// touching the real network.
type HolePunchAttempt struct {
	mu sync.Mutex

	localPort  int
	remoteAddr string

	probes  int
	timeout time.Duration

	state    HolePunchState
	started  time.Time
	probeSeq int

	// conn is the UDP socket used for sending probes.
	conn UDPConn

	// Dialer constructs the UDP socket for the attempt. If nil, a real
	// net.ListenUDP socket bound to the local port is used.
	Dialer func(localPort int) (UDPConn, error)

	// sentPackets records the payloads transmitted (for inspection/testing).
	sentPackets [][]byte
}

// UDPConn is the minimal socket interface HolePunchAttempt needs. It is a
// subset of *net.UDPConn so real and fake connections are interchangeable.
type UDPConn interface {
	WriteToUDP(b []byte, addr *net.UDPAddr) (int, error)
	ReadFromUDP(b []byte) (int, *net.UDPAddr, error)
	Close() error
}

// NewHolePunchAttempt creates a new hole-punch attempt targeting remoteAddr
// from the given local port.
func NewHolePunchAttempt(localPort int, remoteAddr string) *HolePunchAttempt {
	return &HolePunchAttempt{
		localPort: localPort,
		remoteAddr: remoteAddr,
		probes:    DefaultHolePunchProbes,
		timeout:   DefaultHolePunchTimeout,
		state:     HolePunchIdle,
	}
}

// SetProbes overrides the number of probe packets (for testing).
func (h *HolePunchAttempt) SetProbes(n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n > 0 {
		h.probes = n
	}
}

// SetTimeout overrides the attempt timeout (for testing).
func (h *HolePunchAttempt) SetTimeout(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if d > 0 {
		h.timeout = d
	}
}

// State returns the current attempt state.
func (h *HolePunchAttempt) State() HolePunchState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

// SentPackets returns a copy of the probe payloads sent so far.
func (h *HolePunchAttempt) SentPackets() [][]byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([][]byte, len(h.sentPackets))
	for i, p := range h.sentPackets {
		cp := make([]byte, len(p))
		copy(cp, p)
		out[i] = cp
	}
	return out
}

// openSocket resolves the configured Dialer or falls back to a real UDP socket.
func (h *HolePunchAttempt) openSocket() (UDPConn, error) {
	if h.Dialer != nil {
		return h.Dialer(h.localPort)
	}
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", h.localPort))
	if err != nil {
		return nil, fmt.Errorf("resolve local addr: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}
	return conn, nil
}

// AttemptHolePunch performs the hole-punching lifecycle for a single attempt:
// it opens a local UDP socket, sends a burst of probe packets to the remote
// address, and waits briefly for an inbound packet from the peer.
//
// localPort is the local UDP port (as a string, e.g. "0" for an ephemeral
// port chosen by the OS). The function returns nil if the peer responded
// (success) or an error describing the failure otherwise.
func AttemptHolePunch(localPort, remoteAddr string) error {
	port := 0
	if _, err := fmt.Sscanf(localPort, "%d", &port); err != nil {
		return fmt.Errorf("invalid localPort %q: %w", localPort, err)
	}
	h := NewHolePunchAttempt(port, remoteAddr)
	return h.Run()
}

// Run executes the hole-punch lifecycle on an attempt.
func (h *HolePunchAttempt) Run() error {
	h.mu.Lock()
	if h.state != HolePunchIdle {
		h.mu.Unlock()
		return fmt.Errorf("attempt already started (state=%s)", h.state)
	}
	probes := h.probes
	timeout := h.timeout
	h.state = HolePunchPunching
	h.started = time.Now()
	h.mu.Unlock()

	conn, err := h.openSocket()
	if err != nil {
		h.markFailed()
		return err
	}
	defer conn.Close()

	h.mu.Lock()
	h.conn = conn
	h.mu.Unlock()

	remote, err := net.ResolveUDPAddr("udp", h.remoteAddr)
	if err != nil {
		h.markFailed()
		return fmt.Errorf("resolve remote addr: %w", err)
	}

	// Send probe burst.
	for i := 0; i < probes; i++ {
		payload := []byte(fmt.Sprintf("PUNCH-%d", i))
		if _, werr := conn.WriteToUDP(payload, remote); werr != nil {
			h.markFailed()
			return fmt.Errorf("write probe %d: %w", i, werr)
		}
		h.mu.Lock()
		h.probeSeq++
		h.sentPackets = append(h.sentPackets, payload)
		h.mu.Unlock()
	}

	// Wait for any inbound datagram from the peer within the timeout.
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 64)
	for time.Now().Before(deadline) {
		// Best-effort non-blocking-ish read: ReadFromUDP blocks until a packet
		// or socket close. We rely on the socket being closed (and thus the
		// loop ending) only when the caller closes it; here we attempt one
		// read and let the timeout gate via a deadline on the underlying
		// *net.UDPConn.
		if uc, ok := conn.(*net.UDPConn); ok {
			_ = uc.SetReadDeadline(time.Now().Add(time.Until(deadline)))
		}
		n, from, rerr := conn.ReadFromUDP(buf)
		if rerr == nil && n > 0 && from != nil {
			h.mu.Lock()
			h.state = HolePunchSucceeded
			h.mu.Unlock()
			return nil
		}
		if rerr != nil {
			// Timeout or socket closed: stop waiting.
			break
		}
	}

	h.markFailed()
	return fmt.Errorf("hole punch failed: no response from peer after %d probes", probes)
}

func (h *HolePunchAttempt) markFailed() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = HolePunchFailed
}
