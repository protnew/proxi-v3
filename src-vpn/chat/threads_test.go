package chat

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestThreadDepth — builds a 3+ deep chain and verifies depths
// ---------------------------------------------------------------------------

func TestThreadDepth(t *testing.T) {
	t.Parallel()

	tm := NewThreadManager()

	// root -> reply1 -> reply2 -> reply3  (depths 1,2,3,4)
	msgs := []Message{
		{ID: "root", From: "alice", To: "chan1", Text: "root", Ts: 100},
		{ID: "r1", From: "bob", To: "chan1", Text: "reply1", ReplyTo: "root", Ts: 101},
		{ID: "r2", From: "carol", To: "chan1", Text: "reply2", ReplyTo: "r1", Ts: 102},
		{ID: "r3", From: "dave", To: "chan1", Text: "reply3", ReplyTo: "r2", Ts: 103},
	}
	for i := range msgs {
		if err := tm.AddMessage(&msgs[i]); err != nil {
			t.Fatalf("AddMessage(%s): %v", msgs[i].ID, err)
		}
	}

	cases := []struct {
		id    string
		depth int
	}{
		{"root", 1},
		{"r1", 2},
		{"r2", 3},
		{"r3", 4},
		{"nonexistent", 0},
	}
	for _, c := range cases {
		if d := tm.GetThreadDepth(c.id); d != c.depth {
			t.Errorf("GetThreadDepth(%s) = %d, want %d", c.id, d, c.depth)
		}
	}
}

// ---------------------------------------------------------------------------
// TestGetThreadMessages — retrieve direct replies to a parent
// ---------------------------------------------------------------------------

func TestGetThreadMessages(t *testing.T) {
	t.Parallel()

	tm := NewThreadManager()

	// root has two direct replies: c1 and c2.
	_ = tm.AddMessage(&Message{ID: "root2", From: "a", To: "ch", Text: "root", Ts: 1})
	_ = tm.AddMessage(&Message{ID: "c1", From: "b", To: "ch", Text: "child1", ReplyTo: "root2", Ts: 2})
	_ = tm.AddMessage(&Message{ID: "c2", From: "c", To: "ch", Text: "child2", ReplyTo: "root2", Ts: 3})
	// c1 has a nested reply (depth 3).
	_ = tm.AddMessage(&Message{ID: "gc1", From: "d", To: "ch", Text: "grandchild", ReplyTo: "c1", Ts: 4})

	// Direct replies to root2: c1, c2 (sorted).
	direct := tm.GetThreadMessages("root2")
	if len(direct) != 2 {
		t.Fatalf("expected 2 direct replies, got %d", len(direct))
	}
	if direct[0].ID != "c1" || direct[1].ID != "c2" {
		t.Fatalf("expected [c1, c2], got [%s, %s]", direct[0].ID, direct[1].ID)
	}

	// Direct replies to c1: gc1 only.
	nested := tm.GetThreadMessages("c1")
	if len(nested) != 1 || nested[0].ID != "gc1" {
		t.Fatalf("expected [gc1], got %v", nested)
	}

	// No replies to gc1.
	leaf := tm.GetThreadMessages("gc1")
	if len(leaf) != 0 {
		t.Fatalf("expected 0 replies to gc1, got %d", len(leaf))
	}
}

// ---------------------------------------------------------------------------
// TestGetThreadChain — full chain resolution
// ---------------------------------------------------------------------------

func TestGetThreadChain(t *testing.T) {
	t.Parallel()

	tm := NewThreadManager()

	_ = tm.AddMessage(&Message{ID: "top", From: "a", To: "ch", Text: "t", Ts: 1})
	_ = tm.AddMessage(&Message{ID: "mid", From: "b", To: "ch", Text: "m", ReplyTo: "top", Ts: 2})
	_ = tm.AddMessage(&Message{ID: "bot", From: "c", To: "ch", Text: "b", ReplyTo: "mid", Ts: 3})

	th, err := tm.GetThread("bot")
	if err != nil {
		t.Fatalf("GetThread error: %v", err)
	}
	if len(th.Chain) != 3 {
		t.Fatalf("expected chain length 3, got %d", len(th.Chain))
	}
	// Chain should be root-first.
	if th.Chain[0].ID != "top" || th.Chain[1].ID != "mid" || th.Chain[2].ID != "bot" {
		t.Fatalf("chain order wrong: %s %s %s", th.Chain[0].ID, th.Chain[1].ID, th.Chain[2].ID)
	}

	// Unknown message.
	if _, err := tm.GetThread("nope"); err != ErrThreadMessageNotFound {
		t.Fatalf("expected ErrThreadMessageNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestCircularReferenceProtection — adding a reply that forms a cycle is rejected
// ---------------------------------------------------------------------------

func TestCircularReferenceProtection(t *testing.T) {
	t.Parallel()

	tm := NewThreadManager()

	// Build A -> B -> C (C replies to B, B replies to A).
	_ = tm.AddMessage(&Message{ID: "A", From: "a", To: "ch", Text: "A", Ts: 1})
	_ = tm.AddMessage(&Message{ID: "B", From: "b", To: "ch", Text: "B", ReplyTo: "A", Ts: 2})
	_ = tm.AddMessage(&Message{ID: "C", From: "c", To: "ch", Text: "C", ReplyTo: "B", Ts: 3})

	// Attempt: make A reply to C  (A->B->C->A cycle). This must be rejected.
	err := tm.AddMessage(&Message{ID: "A", From: "a", To: "ch", Text: "A-updated", ReplyTo: "C", Ts: 99})
	if err != ErrCircularReference {
		t.Fatalf("expected ErrCircularReference when creating cycle, got %v", err)
	}

	// Attempt: self-reference.
	err = tm.AddMessage(&Message{ID: "selfref", From: "x", To: "ch", Text: "self", ReplyTo: "selfref", Ts: 1})
	if err != ErrCircularReference {
		t.Fatalf("expected ErrCircularReference for self-reference, got %v", err)
	}

	// Attempt: make B reply to C (B is ancestor of C) — also a cycle.
	err = tm.AddMessage(&Message{ID: "B", From: "b", To: "ch", Text: "B-updated", ReplyTo: "C", Ts: 99})
	if err != ErrCircularReference {
		t.Fatalf("expected ErrCircularReference for ancestor-as-child, got %v", err)
	}

	// Valid re-addition without cycle should still work (B replying to A again).
	err = tm.AddMessage(&Message{ID: "B", From: "b", To: "ch", Text: "B-ok", ReplyTo: "A", Ts: 2})
	if err != nil {
		t.Fatalf("expected no error for valid re-add, got %v", err)
	}

	// Depth should still resolve correctly without infinite loop.
	if d := tm.GetThreadDepth("C"); d != 3 {
		t.Fatalf("expected depth 3 for C, got %d", d)
	}
}

// ---------------------------------------------------------------------------
// TestEmptyMessageID — empty ID is rejected
// ---------------------------------------------------------------------------

func TestEmptyMessageID(t *testing.T) {
	t.Parallel()

	tm := NewThreadManager()
	err := tm.AddMessage(&Message{ID: "", From: "a", To: "ch", Text: "x", Ts: 1})
	if err == nil {
		t.Fatal("expected error for empty message ID")
	}
}
