package vpn

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestReconnect_SuccessFirstTry(t *testing.T) {
	rm := NewReconnectManager()
	rm.SetSleepFn(func(time.Duration) {}) // no real sleeping
	rm.SetConnector(func(ctx context.Context) error { return nil })

	if err := rm.Reconnect(); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	if rm.State() != ReconnectConnected {
		t.Errorf("state = %s, want connected", rm.State())
	}
	if rm.Attempts() != 1 {
		t.Errorf("attempts = %d, want 1", rm.Attempts())
	}
}

func TestReconnect_Sequence(t *testing.T) {
	rm := NewReconnectManager()
	rm.SetSleepFn(func(time.Duration) {})

	// Connector fails for the first 3 attempts, then succeeds.
	var calls int32
	rm.SetConnector(func(ctx context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errors.New("transient failure")
		}
		return nil
	})

	if err := rm.Reconnect(); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	if rm.State() != ReconnectConnected {
		t.Errorf("state = %s, want connected", rm.State())
	}
	if rm.Attempts() != 3 {
		t.Errorf("attempts = %d, want 3", rm.Attempts())
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("connector invoked %d times, want 3", calls)
	}
}

func TestReconnect_MaxAttemptsExceeded(t *testing.T) {
	rm := NewReconnectManager()
	rm.SetMaxAttempts(4)
	rm.SetSleepFn(func(time.Duration) {})

	var calls int32
	rm.SetConnector(func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("permanent failure")
	})

	err := rm.Reconnect()
	if err == nil {
		t.Fatal("expected error when all attempts fail")
	}
	if rm.State() != ReconnectGivingUp {
		t.Errorf("state = %s, want giving-up", rm.State())
	}
	if rm.Attempts() != 4 {
		t.Errorf("attempts = %d, want 4", rm.Attempts())
	}
	if atomic.LoadInt32(&calls) != 4 {
		t.Errorf("connector invoked %d times, want 4", calls)
	}
}

func TestReconnect_BackoffSchedule(t *testing.T) {
	rm := NewReconnectManager()
	rm.SetBackoff(100*time.Millisecond, 16*time.Second)
	rm.SetMaxAttempts(5)

	sched := rm.BackoffSchedule()
	// First attempt has no preceding delay, so schedule length = maxAttempts-1.
	if len(sched) != 4 {
		t.Fatalf("schedule length = %d, want 4", len(sched))
	}
	// 100ms, 200ms, 400ms, 800ms (capped by 16s, never reached here).
	want := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
	}
	for i, d := range sched {
		if d != want[i] {
			t.Errorf("schedule[%d] = %v, want %v", i, d, want[i])
		}
	}
}

func TestReconnect_BackoffCapsAtMax(t *testing.T) {
	rm := NewReconnectManager()
	rm.SetBackoff(1*time.Second, 4*time.Second)
	rm.SetMaxAttempts(10)

	sched := rm.BackoffSchedule()
	// Delays should cap at 4s: 1,2,4,4,4,4,4,4,4 (9 entries for 10 attempts).
	for _, d := range sched {
		if d > 4*time.Second {
			t.Errorf("delay %v exceeds max 4s", d)
		}
	}
	// The third delay onward must be the cap.
	if sched[2] != 4*time.Second {
		t.Errorf("schedule[2] = %v, want 4s cap", sched[2])
	}
}

func TestReconnect_CancelledContext(t *testing.T) {
	rm := NewReconnectManager()
	rm.SetSleepFn(func(time.Duration) {})

	// Connector that always fails; context already cancelled so the loop
	// should abort.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rm.SetConnector(func(c context.Context) error {
		return errors.New("nope")
	})

	err := rm.ReconnectCtx(ctx)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if rm.State() != ReconnectGivingUp {
		t.Errorf("state = %s, want giving-up", rm.State())
	}
}

func TestReconnect_Reset(t *testing.T) {
	rm := NewReconnectManager()
	rm.SetSleepFn(func(time.Duration) {})
	rm.SetConnector(func(ctx context.Context) error { return errors.New("fail") })

	_ = rm.Reconnect() // exhausts
	if rm.State() != ReconnectGivingUp {
		t.Fatalf("state = %s, want giving-up", rm.State())
	}

	rm.Reset()
	if rm.State() != ReconnectIdle {
		t.Errorf("after reset state = %s, want idle", rm.State())
	}
	if rm.Attempts() != 0 {
		t.Errorf("after reset attempts = %d, want 0", rm.Attempts())
	}
}

func TestReconnect_StateString(t *testing.T) {
	expectations := map[ReconnectState]string{
		ReconnectIdle:       "idle",
		ReconnectConnecting: "connecting",
		ReconnectConnected:  "connected",
		ReconnectGivingUp:   "giving-up",
	}
	for s, want := range expectations {
		if got := s.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", s, got, want)
		}
	}
}
