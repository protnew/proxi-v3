package vpn

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// ReconnectState describes the state of a ReconnectManager.
type ReconnectState int

const (
	// ReconnectIdle means no reconnection is in progress.
	ReconnectIdle ReconnectState = iota
	// ReconnectConnecting means a reconnection attempt is underway.
	ReconnectConnecting
	// ReconnectConnected means the last reconnection succeeded.
	ReconnectConnected
	// ReconnectGivingUp means the attempt budget was exhausted.
	ReconnectGivingUp
)

func (s ReconnectState) String() string {
	switch s {
	case ReconnectIdle:
		return "idle"
	case ReconnectConnecting:
		return "connecting"
	case ReconnectConnected:
		return "connected"
	case ReconnectGivingUp:
		return "giving-up"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// ReconnectDefaults are the default reconnect parameters.
var (
	DefaultReconnectMaxAttempts = 5
	DefaultReconnectBaseDelay   = 500 * time.Millisecond
	DefaultReconnectMaxDelay    = 16 * time.Second
	// DefaultReconnectJitter is the fractional jitter applied to each delay
	// (0.2 means up to +/-20% of the computed delay).
	DefaultReconnectJitter = 0.2
)

// ReconnectManager coordinates graceful reconnection to the VPN transport with
// exponential backoff and jitter. It is safe for concurrent use.
type ReconnectManager struct {
	mu sync.Mutex

	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	jitter      float64

	// attempts is the number of reconnection attempts in the current cycle.
	attempts int
	state    ReconnectState

	// connectFn performs the actual (re)connection. If nil, Connect always
	// succeeds, which is handy for exercising the backoff sequence in tests.
	connectFn func(ctx context.Context) error

	// sleepFn sleeps for the given duration. Defaults to time.Sleep; tests can
	// substitute a no-op to make Reconnect instant.
	sleepFn func(d time.Duration)

	// nowFn returns the current time. Defaults to time.Now.
	nowFn func() time.Time
}

// NewReconnectManager creates a ReconnectManager with default parameters.
func NewReconnectManager() *ReconnectManager {
	return &ReconnectManager{
		maxAttempts: DefaultReconnectMaxAttempts,
		baseDelay:   DefaultReconnectBaseDelay,
		maxDelay:    DefaultReconnectMaxDelay,
		jitter:      DefaultReconnectJitter,
		sleepFn:     time.Sleep,
		nowFn:       time.Now,
	}
}

// SetConnector installs the connection function invoked on each attempt.
func (rm *ReconnectManager) SetConnector(fn func(ctx context.Context) error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.connectFn = fn
}

// SetSleepFn overrides the sleep function (useful for tests).
func (rm *ReconnectManager) SetSleepFn(fn func(d time.Duration)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if fn != nil {
		rm.sleepFn = fn
	}
}

// SetMaxAttempts overrides the maximum number of attempts.
func (rm *ReconnectManager) SetMaxAttempts(n int) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if n > 0 {
		rm.maxAttempts = n
	}
}

// SetBackoff overrides the base/max backoff delays.
func (rm *ReconnectManager) SetBackoff(base, max time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if base > 0 {
		rm.baseDelay = base
	}
	if max > 0 {
		rm.maxDelay = max
	}
}

// State returns the current reconnect state.
func (rm *ReconnectManager) State() ReconnectState {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.state
}

// Attempts returns the number of reconnection attempts performed in the
// current cycle.
func (rm *ReconnectManager) Attempts() int {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.attempts
}

// Reset returns the manager to its idle state with zero attempts. It should be
// called to begin a fresh reconnection cycle (e.g. after a deliberate
// disconnect).
func (rm *ReconnectManager) Reset() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.attempts = 0
	rm.state = ReconnectIdle
}

// nextDelay computes the exponential backoff delay for the upcoming attempt
// (1-indexed): delay = base * 2^(attempt-1), capped at maxDelay, with optional
// jitter. With jitter disabled the sequence is deterministic.
func (rm *ReconnectManager) nextDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	// Exponential growth, guarded against overflow.
	shift := uint(attempt - 1)
	if shift > 30 {
		shift = 30
	}
	delay := time.Duration(float64(rm.baseDelay) * math.Pow(2, float64(shift)))
	if delay > rm.maxDelay || delay < 0 {
		delay = rm.maxDelay
	}
	return delay
}

// Reconnect attempts to re-establish the connection, retrying with
// exponential backoff up to MaxAttempts times. It returns nil on success or an
// error describing why reconnection ultimately failed (e.g. attempts
// exhausted).
//
// Between attempts the manager sleeps for the backoff delay; tests typically
// replace the sleep function to make Reconnect return quickly.
func (rm *ReconnectManager) Reconnect() error {
	return rm.ReconnectCtx(context.Background())
}

// ReconnectCtx is the context-aware form of Reconnect. A cancelled context
// aborts the reconnection cycle promptly.
func (rm *ReconnectManager) ReconnectCtx(ctx context.Context) error {
	rm.mu.Lock()
	maxAttempts := rm.maxAttempts
	connectFn := rm.connectFn
	sleepFn := rm.sleepFn
	rm.state = ReconnectConnecting
	rm.attempts = 0
	rm.mu.Unlock()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			rm.mu.Lock()
			rm.state = ReconnectGivingUp
			rm.mu.Unlock()
			return fmt.Errorf("reconnect cancelled: %w", err)
		}

		// Sleep before retries (skip the delay on the very first attempt).
		// attempt=2 is retry #1, so its delay is nextDelay(1) = base.
		if attempt > 1 {
			rm.mu.Lock()
			delay := rm.nextDelay(attempt - 1)
			rm.mu.Unlock()
			if sleepFn != nil {
				sleepFn(delay)
			}
		}

		rm.mu.Lock()
		rm.attempts = attempt
		rm.mu.Unlock()

		// Invoke the connector.
		var err error
		if connectFn != nil {
			err = connectFn(ctx)
		}
		if err == nil {
			rm.mu.Lock()
			rm.state = ReconnectConnected
			rm.mu.Unlock()
			return nil
		}
		lastErr = err
	}

	rm.mu.Lock()
	rm.state = ReconnectGivingUp
	rm.mu.Unlock()
	if lastErr == nil {
		lastErr = fmt.Errorf("reconnect failed after %d attempts", maxAttempts)
	}
	return fmt.Errorf("reconnect failed after %d attempts: %w", maxAttempts, lastErr)
}

// BackoffSchedule returns the sequence of delays that would be applied across
// a full cycle (one delay per retry, i.e. maxAttempts-1 entries), which is
// useful for documentation and tests.
func (rm *ReconnectManager) BackoffSchedule() []time.Duration {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	retries := rm.maxAttempts - 1
	if retries < 0 {
		retries = 0
	}
	out := make([]time.Duration, 0, retries)
	for retry := 1; retry <= retries; retry++ {
		out = append(out, rm.nextDelay(retry))
	}
	return out
}
