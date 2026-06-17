package vpn

import (
	"fmt"
	"testing"
)

func TestReplayWindow_FreshAccepted(t *testing.T) {
	rw := NewReplayWindow(DefaultReplayWindowSize)

	// A brand-new cookie must be accepted.
	if !rw.Check([]byte("cookie-1")) {
		t.Fatal("first fresh cookie should be accepted")
	}
	if !rw.Check([]byte("cookie-2")) {
		t.Fatal("second fresh cookie should be accepted")
	}
	if rw.Size() != 2 {
		t.Errorf("Size = %d, want 2", rw.Size())
	}
}

func TestReplayWindow_ReplayRejected(t *testing.T) {
	rw := NewReplayWindow(DefaultReplayWindowSize)

	cookie := []byte("replay-me")
	if !rw.Check(cookie) {
		t.Fatal("first occurrence should be accepted")
	}
	// Submitting the same cookie again must be rejected as a replay.
	if rw.Check(cookie) {
		t.Fatal("duplicate cookie should be rejected as replay")
	}
	if rw.Check(cookie) {
		t.Fatal("third occurrence should also be rejected")
	}
	if rw.Size() != 1 {
		t.Errorf("Size = %d, want 1 (no growth on replay)", rw.Size())
	}
}

func TestReplayWindow_EmptyRejected(t *testing.T) {
	rw := NewReplayWindow(DefaultReplayWindowSize)
	if rw.Check(nil) {
		t.Error("empty cookie should be rejected")
	}
	if rw.Check([]byte{}) {
		t.Error("zero-length cookie should be rejected")
	}
}

func TestReplayWindow_WindowOverflow(t *testing.T) {
	// Small window to exercise the overflow / eviction path.
	const cap = 8
	rw := NewReplayWindow(cap)

	// Fill the window completely.
	for i := 0; i < cap; i++ {
		c := []byte(fmt.Sprintf("c-%d", i))
		if !rw.Check(c) {
			t.Fatalf("cookie %d should be accepted while filling window", i)
		}
	}
	if rw.Size() != cap {
		t.Fatalf("Size = %d, want %d", rw.Size(), cap)
	}

	// All initial cookies must still be considered seen (replays).
	for i := 0; i < cap; i++ {
		c := []byte(fmt.Sprintf("c-%d", i))
		if rw.Check(c) {
			t.Fatalf("cookie %d should be rejected as replay while in window", i)
		}
	}

	// Insert one more cookie, evicting the oldest ("c-0").
	if !rw.Check([]byte("c-new")) {
		t.Fatal("new cookie beyond capacity should be accepted")
	}
	if rw.Size() != cap {
		t.Errorf("Size = %d, want %d (cap bounded)", rw.Size(), cap)
	}

	// The newest original cookie ("c-7") is still in the window, so it must be
	// rejected. (Check this BEFORE re-adding c-0, because accepting c-0 would
	// evict the next-oldest entry c-1.)
	if rw.Check([]byte("c-7")) {
		t.Fatal("cookie c-7 still in window should be rejected")
	}
	if rw.Check([]byte("c-new")) {
		t.Fatal("just-added cookie c-new should be rejected as replay")
	}

	// "c-0" has been evicted, so it should now be accepted again.
	if !rw.Check([]byte("c-0")) {
		t.Fatal("evicted oldest cookie should be accepted again after overflow")
	}
}

func TestReplayWindow_DefaultSize(t *testing.T) {
	rw := NewReplayWindow(0) // should fall back to default
	if rw.cap != DefaultReplayWindowSize {
		t.Errorf("cap = %d, want default %d", rw.cap, DefaultReplayWindowSize)
	}
}
