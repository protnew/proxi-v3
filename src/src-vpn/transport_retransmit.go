package vpn

import (
	"fmt"
	"sync"
	"time"
)

// RetransmitBackoff is the sequence of delays applied between successive
// retransmissions (exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms).
var RetransmitBackoff = []time.Duration{
	100 * time.Millisecond,
	200 * time.Millisecond,
	400 * time.Millisecond,
	800 * time.Millisecond,
	1600 * time.Millisecond,
}

// MaxRetries is the maximum number of retransmission attempts before giving up.
const MaxRetries = 5

// RetransmitTimer tracks retransmission state for a single unacknowledged
// packet. It implements exponential backoff over RetransmitBackoff and gives
// up after MaxRetries attempts.
//
// A RetransmitTimer is NOT safe for concurrent use; it is intended to be
// driven by a single goroutine (e.g. the transport's retransmit loop).
type RetransmitTimer struct {
	mu sync.Mutex

	// attempts is the number of retransmissions already performed (0 means the
	// original packet has not yet been retransmitted).
	attempts int
	maxTries int

	// backoff is the slice of delays to use. It defaults to RetransmitBackoff.
	backoff []time.Duration

	// lastSend is the time of the original send or the most recent retransmit.
	lastSend time.Time
}

// NewRetransmitTimer creates a RetransmitTimer whose clock starts "now".
func NewRetransmitTimer() *RetransmitTimer {
	return NewRetransmitTimerAt(time.Now())
}

// NewRetransmitTimerAt creates a RetransmitTimer whose initial send timestamp
// is set to the provided time. This is primarily useful for deterministic
// tests.
func NewRetransmitTimerAt(now time.Time) *RetransmitTimer {
	return &RetransmitTimer{
		maxTries: MaxRetries,
		backoff:  RetransmitBackoff,
		lastSend: now,
	}
}

// ShouldRetransmit reports whether a retransmission should fire at the given
// time. It returns true only when the retry budget has not been exhausted and
// the delay for the next retry has elapsed since the last send.
//
// ShouldRetransmit does not mutate the timer; call RecordRetransmit after
// actually performing the retransmission.
func (rt *RetransmitTimer) ShouldRetransmit() bool {
	return rt.ShouldRetransmitAt(time.Now())
}

// ShouldRetransmitAt is the clock-injectable form of ShouldRetransmit.
func (rt *RetransmitTimer) ShouldRetransmitAt(now time.Time) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.attempts >= rt.maxTries {
		return false
	}
	delay := rt.backoff[rt.attempts%len(rt.backoff)]
	return now.Sub(rt.lastSend) >= delay
}

// RecordRetransmit marks a retransmission as performed, advancing the attempt
// counter and resetting the send timestamp. It returns the new attempt count
// and an error if the retry budget has been exhausted.
func (rt *RetransmitTimer) RecordRetransmit() (int, error) {
	return rt.RecordRetransmitAt(time.Now())
}

// RecordRetransmitAt is the clock-injectable form of RecordRetransmit.
func (rt *RetransmitTimer) RecordRetransmitAt(now time.Time) (int, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.attempts >= rt.maxTries {
		return rt.attempts, fmt.Errorf("max retries (%d) exceeded", rt.maxTries)
	}
	rt.attempts++
	rt.lastSend = now
	return rt.attempts, nil
}

// Attempts returns the number of retransmissions performed so far.
func (rt *RetransmitTimer) Attempts() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.attempts
}

// Exhausted reports whether the retry budget has been fully consumed.
func (rt *RetransmitTimer) Exhausted() bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.attempts >= rt.maxTries
}

// Reset returns the timer to its initial state (zero attempts) and resets the
// send timestamp to now. It should be called when an ACK is received or the
// packet is otherwise acknowledged.
func (rt *RetransmitTimer) Reset() {
	rt.ResetAt(time.Now())
}

// ResetAt is the clock-injectable form of Reset.
func (rt *RetransmitTimer) ResetAt(now time.Time) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.attempts = 0
	rt.lastSend = now
}

// NextBackoff returns the delay that will be waited before the next
// retransmission, or zero if the budget is exhausted.
func (rt *RetransmitTimer) NextBackoff() time.Duration {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.attempts >= rt.maxTries {
		return 0
	}
	return rt.backoff[rt.attempts%len(rt.backoff)]
}
