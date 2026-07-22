package vpn

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

// DefaultReplayWindowSize is the default number of cookie hashes tracked.
const DefaultReplayWindowSize = 1000

// hashSize is the number of bytes of the SHA-256 digest we store per cookie.
// 16 bytes (128 bits) is more than enough collision resistance for replay
// detection while keeping memory usage bounded.
const hashSize = 16

// ReplayWindow is a fixed-capacity ring buffer that tracks the hashes of the
// most recently seen cookies. It is used to detect and reject replayed packets:
// a cookie that has already been observed within the window is rejected.
//
// The implementation is safe for concurrent use.
type ReplayWindow struct {
	mu sync.Mutex

	// ring is a circular buffer of capacity 'cap' holding the most recent
	// cookie hashes in insertion order (oldest at head, newest at tail).
	ring [][hashSize]byte
	head int // index of oldest element
	size int // number of elements currently stored
	cap  int // maximum number of elements

	// seen allows O(1) membership checks against the currently buffered hashes.
	seen map[[hashSize]byte]struct{}
}

// NewReplayWindow creates a ReplayWindow with the given capacity.
// If cap <= 0, DefaultReplayWindowSize is used.
func NewReplayWindow(capacity int) *ReplayWindow {
	if capacity <= 0 {
		capacity = DefaultReplayWindowSize
	}
	return &ReplayWindow{
		ring: make([][hashSize]byte, capacity),
		cap:  capacity,
		seen: make(map[[hashSize]byte]struct{}, capacity),
	}
}

// hashCookie computes a fixed-size digest of a cookie using SHA-256 truncated
// to hashSize bytes.
func hashCookie(cookie []byte) [hashSize]byte {
	sum := sha256.Sum256(cookie)
	var out [hashSize]byte
	copy(out[:], sum[:hashSize])
	return out
}

// Check verifies whether the given cookie is fresh (not seen before within the
// window). If it is fresh, the cookie hash is recorded and Check returns true.
// If the cookie has already been seen (a replay), Check returns false.
//
// An empty cookie is always rejected.
func (rw *ReplayWindow) Check(cookie []byte) bool {
	if len(cookie) == 0 {
		return false
	}

	h := hashCookie(cookie)

	rw.mu.Lock()
	defer rw.mu.Unlock()

	if _, ok := rw.seen[h]; ok {
		// Replay detected: cookie already in the window.
		return false
	}

	// Fresh cookie: record it.
	if rw.size == rw.cap {
		// Buffer full: evict the oldest entry.
		oldest := rw.ring[rw.head]
		delete(rw.seen, oldest)
		rw.ring[rw.head] = h
		rw.head = (rw.head + 1) % rw.cap
	} else {
		// Buffer not yet full: append to the tail.
		tail := (rw.head + rw.size) % rw.cap
		rw.ring[tail] = h
		rw.size++
	}
	rw.seen[h] = struct{}{}
	return true
}

// Size returns the number of cookie hashes currently stored in the window.
func (rw *ReplayWindow) Size() int {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.size
}

// String renders the window state for debugging.
func (rw *ReplayWindow) String() string {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return fmt.Sprintf("ReplayWindow(size=%d, cap=%d)", rw.size, rw.cap)
}
