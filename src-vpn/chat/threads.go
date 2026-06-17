package chat

import (
	"errors"
	"sync"
)

// maxThreadDepth is a safety cap to prevent runaway cycle detection / traversal.
const maxThreadDepth = 10000

// ErrThreadMessageNotFound is returned when a thread operation references an
// unknown message ID.
var ErrThreadMessageNotFound = errors.New("thread message not found")

// ErrCircularReference is returned when adding a reply that would create a
// circular parent chain.
var ErrCircularReference = errors.New("circular reference detected in thread chain")

// Thread represents a resolved reply chain for a message. The Chain field
// lists messages from the root ancestor down to (and including) the target
// message, in chronological/topological order.
type Thread struct {
	TargetID string    // ID of the message the thread was resolved for
	Chain    []Message // root … → … → target message
}

// ThreadManager stores messages and their reply relationships, supporting
// multi-level reply chains (depth > 1). It is safe for concurrent use.
type ThreadManager struct {
	mu       sync.RWMutex
	msgs     map[string]*Message          // msgID → message
	children map[string]map[string]bool   // parentID → set of child msgIDs
	parentOf map[string]string            // childID → parentID (ReplyTo cache)
}

// NewThreadManager creates a new empty ThreadManager.
func NewThreadManager() *ThreadManager {
	return &ThreadManager{
		msgs:     make(map[string]*Message),
		children: make(map[string]map[string]bool),
		parentOf: make(map[string]string),
	}
}

// AddMessage registers a message in the thread manager. If the message has a
// non-empty ReplyTo, a cycle check is performed to prevent circular references.
// A message whose ReplyTo is not yet registered is still accepted; the parent
// chain is resolved lazily as messages are added.
func (tm *ThreadManager) AddMessage(msg *Message) error {
	if msg.ID == "" {
		return errors.New("message ID must not be empty")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// If the message references a parent, ensure adding it does not create a
	// cycle. A self-reference or any ancestor loop is rejected.
	if msg.ReplyTo != "" {
		if msg.ReplyTo == msg.ID || tm.wouldCreateCycleLocked(msg.ID, msg.ReplyTo) {
			return ErrCircularReference
		}
	}

	// Store a defensive copy so external mutation doesn't corrupt state.
	cp := *msg
	if cp.Ts == 0 {
		// Leave Ts as-is if zero; callers may set it.
	}
	tm.msgs[msg.ID] = &cp

	// Record parent/child relationship.
	if msg.ReplyTo != "" && msg.ReplyTo != msg.ID {
		tm.parentOf[msg.ID] = msg.ReplyTo
		chSet, ok := tm.children[msg.ReplyTo]
		if !ok {
			chSet = make(map[string]bool)
			tm.children[msg.ReplyTo] = chSet
		}
		chSet[msg.ID] = true
	}

	return nil
}

// wouldCreateCycleLocked checks whether making childID a reply to parentID
// would create a circular chain. This happens when parentID is reachable by
// walking *up* from childID (i.e. childID is already an ancestor of parentID),
// or when parentID == childID. Caller must hold tm.mu.
func (tm *ThreadManager) wouldCreateCycleLocked(childID, parentID string) bool {
	if childID == parentID {
		return true
	}
	// Walk up from parentID; if we ever reach childID, a cycle would form.
	visited := make(map[string]bool)
	cur := parentID
	for cur != "" {
		if cur == childID {
			return true
		}
		if visited[cur] {
			// Existing data has a cycle — treat as dangerous.
			return true
		}
		visited[cur] = true
		var ok bool
		cur, ok = tm.parentOf[cur]
		if !ok {
			break
		}
	}
	return false
}

// GetThreadDepth returns the depth of a message in its reply chain.
// A root message (no parent) has depth 1; a direct reply has depth 2; etc.
// Returns 0 if the message is not registered.
// Circular references in the chain are detected and capped.
func (tm *ThreadManager) GetThreadDepth(msgID string) int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if _, ok := tm.msgs[msgID]; !ok {
		return 0
	}

	depth := 1
	visited := make(map[string]bool)
	cur := msgID
	for {
		parentID, ok := tm.parentOf[cur]
		if !ok || parentID == "" {
			break
		}
		if visited[cur] {
			// Cycle in existing data — stop to avoid infinite loop.
			break
		}
		visited[cur] = true
		depth++
		cur = parentID
		if depth > maxThreadDepth {
			break
		}
	}
	return depth
}

// GetThreadMessages returns all direct replies to the given parent message ID.
// The returned slice is ordered by message ID for determinism.
// Returns an empty slice if the parent has no replies or is unknown.
func (tm *ThreadManager) GetThreadMessages(parentID string) []Message {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	chSet, ok := tm.children[parentID]
	if !ok || len(chSet) == 0 {
		return []Message{}
	}

	// Collect child IDs and sort for determinism.
	ids := make([]string, 0, len(chSet))
	for id := range chSet {
		ids = append(ids, id)
	}
	sortStrings(ids)

	result := make([]Message, 0, len(ids))
	for _, id := range ids {
		if m, ok := tm.msgs[id]; ok {
			result = append(result, *m)
		}
	}
	return result
}

// GetThread resolves the full parent chain for the given message, returning a
// Thread containing the chain from root to the target. Returns
// ErrThreadMessageNotFound if the message is not registered.
func (tm *ThreadManager) GetThread(msgID string) (*Thread, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if _, ok := tm.msgs[msgID]; !ok {
		return nil, ErrThreadMessageNotFound
	}

	// Walk up collecting ancestors, then reverse.
	var chain []Message
	visited := make(map[string]bool)
	cur := msgID
	steps := 0
	for cur != "" {
		if visited[cur] {
			break // cycle protection
		}
		visited[cur] = true
		m, ok := tm.msgs[cur]
		if !ok {
			break
		}
		chain = append(chain, *m)
		parentID, hasParent := tm.parentOf[cur]
		if !hasParent {
			break
		}
		cur = parentID
		steps++
		if steps > maxThreadDepth {
			break
		}
	}

	// Reverse so root is first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return &Thread{TargetID: msgID, Chain: chain}, nil
}

// MessageCount returns the total number of registered messages.
func (tm *ThreadManager) MessageCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.msgs)
}

// sortStrings performs an insertion sort on a string slice (small N in tests).
func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}
