package chat

import (
	"errors"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// TestQueuedToSent — legal forward transition
// ---------------------------------------------------------------------------

func TestQueuedToSent(t *testing.T) {
	t.Parallel()

	msgID := "trans-queued-sent"
	defer statusStore.Delete(msgID)

	// Initial status is Queued.
	if s := GetMessageStatus(msgID); s != Queued {
		t.Fatalf("expected Queued, got %s", statusName(s))
	}

	if err := TransitionStatus(msgID, Sent); err != nil {
		t.Fatalf("Queued→Sent error: %v", err)
	}
	if s := GetMessageStatus(msgID); s != Sent {
		t.Fatalf("expected Sent, got %s", statusName(s))
	}

	// Idempotent: Sent→Sent is a no-op success.
	if err := TransitionStatus(msgID, Sent); err != nil {
		t.Fatalf("Sent→Sent should be idempotent, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestSentToDelivered
// ---------------------------------------------------------------------------

func TestSentToDelivered(t *testing.T) {
	t.Parallel()

	msgID := "trans-sent-delivered"
	defer statusStore.Delete(msgID)

	SetMessageStatus(msgID, Sent)

	if err := TransitionStatus(msgID, Delivered); err != nil {
		t.Fatalf("Sent→Delivered error: %v", err)
	}
	if s := GetMessageStatus(msgID); s != Delivered {
		t.Fatalf("expected Delivered, got %s", statusName(s))
	}
}

// ---------------------------------------------------------------------------
// TestDeliveredToRead
// ---------------------------------------------------------------------------

func TestDeliveredToRead(t *testing.T) {
	t.Parallel()

	msgID := "trans-delivered-read"
	defer statusStore.Delete(msgID)

	SetMessageStatus(msgID, Delivered)

	if err := TransitionStatus(msgID, Read); err != nil {
		t.Fatalf("Delivered→Read error: %v", err)
	}
	if s := GetMessageStatus(msgID); s != Read {
		t.Fatalf("expected Read, got %s", statusName(s))
	}
}

// ---------------------------------------------------------------------------
// TestFullForwardChain — Queued→Sent→Delivered→Read end to end
// ---------------------------------------------------------------------------

func TestFullForwardChain(t *testing.T) {
	t.Parallel()

	msgID := "trans-full-chain"
	defer statusStore.Delete(msgID)

	for _, target := range []MessageStatus{Sent, Delivered, Read} {
		if err := TransitionStatus(msgID, target); err != nil {
			t.Fatalf("transition to %s failed: %v", statusName(target), err)
		}
	}
	if s := GetMessageStatus(msgID); s != Read {
		t.Fatalf("expected Read, got %s", statusName(s))
	}
}

// ---------------------------------------------------------------------------
// TestInvalidTransition — backwards and skip-step transitions rejected
// ---------------------------------------------------------------------------

func TestInvalidTransition(t *testing.T) {
	t.Parallel()

	msgID := "trans-invalid"
	defer statusStore.Delete(msgID)

	// Start at Delivered.
	SetMessageStatus(msgID, Delivered)

	// Backwards: Delivered→Sent is invalid.
	err := TransitionStatus(msgID, Sent)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Delivered→Sent should be invalid, got: %v", err)
	}
	// Status must be unchanged.
	if s := GetMessageStatus(msgID); s != Delivered {
		t.Fatalf("status changed after invalid transition: %s", statusName(s))
	}

	// Backwards: Delivered→Queued invalid.
	if err := TransitionStatus(msgID, Queued); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Delivered→Queued should be invalid, got: %v", err)
	}

	// Skip step from Queued: set to Queued then try Queued→Delivered.
	SetMessageStatus(msgID, Queued)
	if err := TransitionStatus(msgID, Delivered); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Queued→Delivered (skip) should be invalid, got: %v", err)
	}
	if err := TransitionStatus(msgID, Read); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Queued→Read (skip) should be invalid, got: %v", err)
	}

	// Status still Queued.
	if s := GetMessageStatus(msgID); s != Queued {
		t.Fatalf("expected Queued after invalid attempts, got %s", statusName(s))
	}

	// Empty message ID.
	if err := TransitionStatus("", Sent); err == nil {
		t.Fatal("expected error for empty message ID")
	}
}

// ---------------------------------------------------------------------------
// TestConcurrentTransition — concurrent legal transitions stay consistent
// ---------------------------------------------------------------------------

func TestConcurrentTransition(t *testing.T) {
	t.Parallel()

	msgID := "trans-concurrent"
	defer statusStore.Delete(msgID)

	const N = 100
	var wg sync.WaitGroup
	var successCount, idempotentCount int64
	var mu sync.Mutex

	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			err := TransitionStatus(msgID, Sent)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			}
		}()
	}
	wg.Wait()

	// All goroutines should succeed (first transitions, rest are idempotent).
	if successCount != N {
		t.Fatalf("expected %d successes, got %d", N, successCount)
	}
	// Final status must be Sent — no corruption.
	if s := GetMessageStatus(msgID); s != Sent {
		t.Fatalf("expected Sent after concurrent transitions, got %s", statusName(s))
	}
	_ = idempotentCount

	// Now concurrently advance Sent→Delivered: verify consistency again.
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_ = TransitionStatus(msgID, Delivered)
		}()
	}
	wg.Wait()
	if s := GetMessageStatus(msgID); s != Delivered {
		t.Fatalf("expected Delivered after concurrent transitions, got %s", statusName(s))
	}
}

// ---------------------------------------------------------------------------
// TestConcurrentMixedTransitions — valid + invalid transitions don't corrupt state
// ---------------------------------------------------------------------------

func TestConcurrentMixedTransitions(t *testing.T) {
	t.Parallel()

	msgID := "trans-concurrent-mixed"
	defer statusStore.Delete(msgID)

	const N = 50
	var wg sync.WaitGroup

	wg.Add(N * 2)
	// Half try valid Queued→Sent.
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_ = TransitionStatus(msgID, Sent)
		}()
	}
	// Half try invalid Queued→Read (skip-step).
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_ = TransitionStatus(msgID, Read)
		}()
	}
	wg.Wait()

	// Final status must be Sent (the only legal target from Queued), never Read.
	s := GetMessageStatus(msgID)
	if s != Sent {
		t.Fatalf("expected Sent after mixed concurrent transitions, got %s", statusName(s))
	}
}
