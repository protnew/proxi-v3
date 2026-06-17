package vpn

import (
	"testing"
	"time"
)

func TestRetransmit_Sequence(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	rt := NewRetransmitTimerAt(start)

	// Immediately after the original send, no retransmit should fire.
	if rt.ShouldRetransmitAt(start) {
		t.Error("should not retransmit immediately after send")
	}
	if rt.Exhausted() {
		t.Error("timer should not be exhausted initially")
	}

	// Walk through each backoff step. Backoff is measured relative to the most
	// recent send, so we maintain a running cursor that advances by each step's
	// delay (the lastSend moves with every RecordRetransmit).
	cur := start
	for i, d := range RetransmitBackoff {
		// Just before the delay elapses (relative to last send): not yet.
		if rt.ShouldRetransmitAt(cur.Add(d - 1)) {
			t.Errorf("step %d: should not retransmit before backoff (%v)", i, d-1)
		}
		// At the delay: should retransmit.
		fireAt := cur.Add(d)
		if !rt.ShouldRetransmitAt(fireAt) {
			t.Errorf("step %d: should retransmit after backoff %v", i, d)
		}
		n, err := rt.RecordRetransmitAt(fireAt)
		if err != nil {
			t.Fatalf("step %d: RecordRetransmitAt error: %v", i, err)
		}
		if n != i+1 {
			t.Errorf("step %d: attempts = %d, want %d", i, n, i+1)
		}
		cur = fireAt // lastSend has now moved here
	}

	// After exactly MaxRetries attempts, the budget is exhausted.
	if !rt.Exhausted() {
		t.Error("timer should be exhausted after MaxRetries retransmits")
	}
	if rt.Attempts() != MaxRetries {
		t.Errorf("attempts = %d, want %d", rt.Attempts(), MaxRetries)
	}
}

func TestRetransmit_MaxRetriesExceeded(t *testing.T) {
	start := time.Unix(2_000_000, 0)
	rt := NewRetransmitTimerAt(start)

	// Perform the full quota of retransmits.
	for i := 0; i < MaxRetries; i++ {
		d := rt.NextBackoff()
		if d <= 0 {
			t.Fatalf("unexpected zero backoff at step %d", i)
		}
		if _, err := rt.RecordRetransmitAt(start.Add(d)); err != nil {
			t.Fatalf("step %d: unexpected error: %v", i, err)
		}
	}

	// Beyond the budget: ShouldRetransmit must be false.
	if rt.ShouldRetransmitAt(start.Add(time.Hour)) {
		t.Error("should not retransmit once retries are exhausted")
	}
	// RecordRetransmit must return an error and not advance the count.
	before := rt.Attempts()
	if _, err := rt.RecordRetransmitAt(start.Add(time.Hour)); err == nil {
		t.Error("expected error when recording retransmit after exhaustion")
	}
	if rt.Attempts() != before {
		t.Errorf("attempts changed after exhaustion: %d -> %d", before, rt.Attempts())
	}
	if rt.NextBackoff() != 0 {
		t.Errorf("NextBackoff after exhaustion = %v, want 0", rt.NextBackoff())
	}
}

func TestRetransmit_Reset(t *testing.T) {
	start := time.Unix(3_000_000, 0)
	rt := NewRetransmitTimerAt(start)

	// Use up some retries.
	for i := 0; i < 3; i++ {
		_, _ = rt.RecordRetransmitAt(start.Add(RetransmitBackoff[i]))
	}
	if rt.Attempts() != 3 {
		t.Fatalf("attempts = %d, want 3", rt.Attempts())
	}

	// Reset on ACK.
	rt.ResetAt(start.Add(time.Second))
	if rt.Attempts() != 0 {
		t.Errorf("after reset attempts = %d, want 0", rt.Attempts())
	}
	if rt.Exhausted() {
		t.Error("timer should not be exhausted after reset")
	}
	// Fresh backoff should be the first step again.
	if rt.NextBackoff() != RetransmitBackoff[0] {
		t.Errorf("after reset NextBackoff = %v, want %v", rt.NextBackoff(), RetransmitBackoff[0])
	}
}
