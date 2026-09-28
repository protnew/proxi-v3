package chat

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestPinUnpin — pin a message, get it, unpin, verify removal
// ---------------------------------------------------------------------------

func TestPinUnpin(t *testing.T) {
	t.Parallel()

	pm := NewPinManager()

	// Pin a message.
	p := pm.Pin("msg-001", "channel-A", "user-alice")
	if p.MsgID != "msg-001" {
		t.Fatalf("expected msgID=msg-001, got %s", p.MsgID)
	}
	if p.ChannelID != "channel-A" {
		t.Fatalf("expected channelID=channel-A, got %s", p.ChannelID)
	}
	if p.PinnedBy != "user-alice" {
		t.Fatalf("expected pinnedBy=user-alice, got %s", p.PinnedBy)
	}
	if p.PinnedAt == 0 {
		t.Fatal("expected nonzero PinnedAt")
	}

	// IsPinned should be true.
	if !pm.IsPinned("msg-001") {
		t.Fatal("expected message to be pinned")
	}

	// GetPinned should return the record.
	got, err := pm.GetPinned("msg-001")
	if err != nil {
		t.Fatalf("GetPinned returned error: %v", err)
	}
	if got.MsgID != "msg-001" {
		t.Fatalf("GetPinned returned wrong msgID: %s", got.MsgID)
	}

	// Unpin.
	if err := pm.Unpin("msg-001", "channel-A"); err != nil {
		t.Fatalf("Unpin returned error: %v", err)
	}
	if pm.IsPinned("msg-001") {
		t.Fatal("expected message to be unpinned after Unpin")
	}

	// GetPinned after unpin should error.
	if _, err := pm.GetPinned("msg-001"); err != ErrPinnedNotFound {
		t.Fatalf("expected ErrPinnedNotFound after unpin, got %v", err)
	}

	// Unpin again should error.
	if err := pm.Unpin("msg-001", "channel-A"); err != ErrPinnedNotFound {
		t.Fatalf("expected ErrPinnedNotFound on double unpin, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestListPinned — multiple pins in a channel, filtering, cross-channel
// ---------------------------------------------------------------------------

func TestListPinned(t *testing.T) {
	t.Parallel()

	pm := NewPinManager()

	// Channel A has two pinned messages.
	pm.Pin("a-001", "channel-A", "user-1")
	pm.Pin("a-002", "channel-A", "user-2")
	// Channel B has one.
	pm.Pin("b-001", "channel-B", "user-3")

	// List channel A — expect 2.
	listA := pm.ListPinned("channel-A")
	if len(listA) != 2 {
		t.Fatalf("expected 2 pinned in channel-A, got %d", len(listA))
	}
	// All should belong to channel-A.
	for _, p := range listA {
		if p.ChannelID != "channel-A" {
			t.Fatalf("expected channel-A, got %s", p.ChannelID)
		}
	}
	// Ordering: should be sorted by PinnedAt asc — a-001 before a-002.
	if listA[0].MsgID != "a-001" || listA[1].MsgID != "a-002" {
		t.Fatalf("expected order [a-001, a-002], got [%s, %s]", listA[0].MsgID, listA[1].MsgID)
	}

	// List channel B — expect 1.
	listB := pm.ListPinned("channel-B")
	if len(listB) != 1 {
		t.Fatalf("expected 1 pinned in channel-B, got %d", len(listB))
	}
	if listB[0].MsgID != "b-001" {
		t.Fatalf("expected b-001, got %s", listB[0].MsgID)
	}

	// Empty channel — expect empty slice, not nil.
	listEmpty := pm.ListPinned("channel-none")
	if len(listEmpty) != 0 {
		t.Fatalf("expected 0 pinned in unknown channel, got %d", len(listEmpty))
	}

	// Total count.
	if pm.Count() != 3 {
		t.Fatalf("expected total count 3, got %d", pm.Count())
	}

	// Unpin one from channel-A and re-check.
	_ = pm.Unpin("a-001", "channel-A")
	listA2 := pm.ListPinned("channel-A")
	if len(listA2) != 1 || listA2[0].MsgID != "a-002" {
		t.Fatalf("after unpinning a-001, expected [a-002], got %v", listA2)
	}
}

// ---------------------------------------------------------------------------
// TestDuplicatePinSafe — pinning the same message twice is idempotent
// ---------------------------------------------------------------------------

func TestDuplicatePinSafe(t *testing.T) {
	t.Parallel()

	pm := NewPinManager()

	p1 := pm.Pin("dup-001", "channel-D", "user-x")

	// Pin the same message again, possibly by a different user.
	p2 := pm.Pin("dup-001", "channel-D", "user-y")

	// The original metadata should be preserved (PinnedBy, PinnedAt unchanged).
	if p1.PinnedBy != p2.PinnedBy {
		t.Fatalf("duplicate pin should preserve PinnedBy: %s vs %s", p1.PinnedBy, p2.PinnedBy)
	}
	if p1.PinnedAt != p2.PinnedAt {
		t.Fatalf("duplicate pin should preserve PinnedAt: %d vs %d", p1.PinnedAt, p2.PinnedAt)
	}

	// Only one entry should exist.
	if pm.Count() != 1 {
		t.Fatalf("expected 1 pin after duplicate pin, got %d", pm.Count())
	}
	if len(pm.ListPinned("channel-D")) != 1 {
		t.Fatalf("expected 1 pin in channel-D after duplicate, got %d", len(pm.ListPinned("channel-D")))
	}
}
