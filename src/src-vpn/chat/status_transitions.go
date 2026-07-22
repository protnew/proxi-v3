package chat

import (
	"errors"
	"fmt"
	"sync"
)

// ErrInvalidTransition is returned when a status transition is not allowed.
var ErrInvalidTransition = errors.New("invalid message status transition")

// transitionLock serializes check-and-set transitions so that concurrent
// TransitionStatus calls on the same message are race-free.
var transitionLock sync.Mutex

// TransitionStatus atomically moves a message from its current status to
// newStatus, enforcing the legal state machine:
//
//	Queued → Sent → Delivered → Read
//
// Rules:
//   - Transitioning to the same status (e.g. Sent → Sent) is an idempotent
//     no-op and returns nil.
//   - Only a single forward step is allowed (current+1).
//   - Any other transition (backwards or skipping a step) returns
//     ErrInvalidTransition and leaves the status unchanged.
func TransitionStatus(msgID string, newStatus MessageStatus) error {
	if msgID == "" {
		return errors.New("message ID must not be empty")
	}

	transitionLock.Lock()
	defer transitionLock.Unlock()

	current := GetMessageStatus(msgID)

	// Idempotent no-op.
	if current == newStatus {
		return nil
	}

	// Only a single forward step is valid.
	if newStatus != current+1 {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, statusName(current), statusName(newStatus))
	}

	SetMessageStatus(msgID, newStatus)
	return nil
}

// statusName returns a human-readable name for a MessageStatus.
func statusName(s MessageStatus) string {
	switch s {
	case Queued:
		return "Queued"
	case Sent:
		return "Sent"
	case Delivered:
		return "Delivered"
	case Read:
		return "Read"
	default:
		return fmt.Sprintf("Unknown(%d)", int(s))
	}
}
