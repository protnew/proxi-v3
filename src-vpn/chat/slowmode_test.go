package chat

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestSlowModeRateLimit — first message allowed, second blocked, then allowed after interval
// ---------------------------------------------------------------------------

func TestSlowModeRateLimit(t *testing.T) {
	t.Parallel()

	sm := NewSlowModeManager()
	sm.SetSlowMode("channel-R", 100*time.Millisecond)

	// First message: allowed.
	if !sm.CanSend("user1", "channel-R") {
		t.Fatal("first message should be allowed")
	}
	sm.RecordSend("user1", "channel-R")

	// Immediate second message: blocked by rate limit.
	if sm.CanSend("user1", "channel-R") {
		t.Fatal("second message should be blocked by slow mode")
	}

	// TimeUntilCanSend should be positive.
	if wait := sm.TimeUntilCanSend("user1", "channel-R"); wait <= 0 {
		t.Fatalf("expected positive wait time, got %v", wait)
	}

	// Wait for the interval to pass.
	time.Sleep(110 * time.Millisecond)

	// Now allowed again (interval reset).
	if !sm.CanSend("user1", "channel-R") {
		t.Fatal("message should be allowed after interval elapsed")
	}
}

// ---------------------------------------------------------------------------
// TestSlowModePerUserIsolation — one user's rate limit doesn't affect another
// ---------------------------------------------------------------------------

func TestSlowModePerUserIsolation(t *testing.T) {
	t.Parallel()

	sm := NewSlowModeManager()
	sm.SetSlowMode("channel-I", 100*time.Millisecond)

	// user-A sends.
	sm.RecordSend("user-A", "channel-I")
	// user-A is now rate-limited.
	if sm.CanSend("user-A", "channel-I") {
		t.Fatal("user-A should be rate limited")
	}
	// user-B is unaffected.
	if !sm.CanSend("user-B", "channel-I") {
		t.Fatal("user-B should NOT be rate limited")
	}
}

// ---------------------------------------------------------------------------
// TestSlowModeAdminExempt — admins bypass rate limiting
// ---------------------------------------------------------------------------

func TestSlowModeAdminExempt(t *testing.T) {
	t.Parallel()

	sm := NewSlowModeManager()
	sm.SetSlowMode("channel-A", 1*time.Hour) // very long interval
	sm.SetAdmin("admin1", true)

	// Admin sends once.
	if !sm.CanSend("admin1", "channel-A") {
		t.Fatal("admin should be allowed (first message)")
	}
	sm.RecordSend("admin1", "channel-A")

	// Admin sends again immediately — still allowed (exempt).
	if !sm.CanSend("admin1", "channel-A") {
		t.Fatal("admin should be exempt from slow mode")
	}
	if wait := sm.TimeUntilCanSend("admin1", "channel-A"); wait != 0 {
		t.Fatalf("admin wait should be 0, got %v", wait)
	}

	// Regular user is still rate-limited after sending.
	sm.RecordSend("regular1", "channel-A")
	if sm.CanSend("regular1", "channel-A") {
		t.Fatal("regular user should be rate limited")
	}

	// Remove admin status — now subject to slow mode.
	sm.SetAdmin("admin1", false)
	if sm.CanSend("admin1", "channel-A") {
		// admin1 already sent once, so should be limited now.
		t.Fatal("admin1 should be rate-limited after losing admin status")
	}
}

// ---------------------------------------------------------------------------
// TestSlowModeNoSlowMode — channels without slow mode allow everything
// ---------------------------------------------------------------------------

func TestSlowModeNoSlowMode(t *testing.T) {
	t.Parallel()

	sm := NewSlowModeManager()

	// No slow mode set on channel-N.
	if sm.GetSlowMode("channel-N") != 0 {
		t.Fatal("expected 0 interval for channel without slow mode")
	}
	// Multiple sends all allowed.
	for i := 0; i < 10; i++ {
		if !sm.CanSend("user1", "channel-N") {
			t.Fatalf("message %d should be allowed without slow mode", i)
		}
		sm.RecordSend("user1", "channel-N")
	}
}

// ---------------------------------------------------------------------------
// TestSlowModeDisable — SetSlowMode with 0 disables it
// ---------------------------------------------------------------------------

func TestSlowModeDisable(t *testing.T) {
	t.Parallel()

	sm := NewSlowModeManager()
	sm.SetSlowMode("channel-D", 1*time.Hour)
	sm.RecordSend("user1", "channel-D")
	if sm.CanSend("user1", "channel-D") {
		t.Fatal("should be rate limited with 1h interval")
	}

	// Disable slow mode.
	sm.SetSlowMode("channel-D", 0)
	if sm.GetSlowMode("channel-D") != 0 {
		t.Fatal("expected 0 interval after disable")
	}
	if !sm.CanSend("user1", "channel-D") {
		t.Fatal("should be allowed after slow mode disabled")
	}
}

// ---------------------------------------------------------------------------
// TestSlowModeResetUser — manual reset allows immediate send
// ---------------------------------------------------------------------------

func TestSlowModeResetUser(t *testing.T) {
	t.Parallel()

	sm := NewSlowModeManager()
	sm.SetSlowMode("channel-X", 1*time.Hour)
	sm.RecordSend("user1", "channel-X")
	if sm.CanSend("user1", "channel-X") {
		t.Fatal("should be rate limited")
	}

	sm.ResetUser("user1", "channel-X")
	if !sm.CanSend("user1", "channel-X") {
		t.Fatal("should be allowed after reset")
	}
}
