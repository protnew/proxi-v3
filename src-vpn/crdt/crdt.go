// Package crdt implements Conflict-free Replicated Data Types for distributed
// state synchronization across relay nodes. Three CRDTs are provided:
//
//   - LWWRegister: Last-Writer-Wins register for single-value state
//   - GCounter: Grow-only counter (increment-only, merge by max per node)
//   - ORSet: Observed-Remove Set (add/remove elements, merge by observed-remove semantics)
package crdt

import (
	"sync"
)

// ==================== LWWRegister ====================

// LWWRegister is a Last-Writer-Wins CRDT register.
// It holds a single value tagged with a timestamp. On merge, the value with
// the higher timestamp wins. Ties are broken by lexicographic comparison of values.
type LWWRegister struct {
	mu        sync.RWMutex
	key       string
	value     []byte
	timestamp int64
}

// NewLWWRegister creates a new LWWRegister with the given key and zero initial state.
func NewLWWRegister(key string) *LWWRegister {
	return &LWWRegister{
		key:       key,
		value:     nil,
		timestamp: 0,
	}
}

// Key returns the register's key.
func (r *LWWRegister) Key() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.key
}

// Set updates the register value if the provided timestamp is greater than the
// current timestamp. If the timestamps are equal, the lexicographically larger
// value wins (deterministic tie-breaking).
func (r *LWWRegister) Set(value []byte, ts int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ts > r.timestamp {
		r.value = make([]byte, len(value))
		copy(r.value, value)
		r.timestamp = ts
	} else if ts == r.timestamp {
		// Tie-breaking: lexicographic comparison
		if compareBytes(value, r.value) > 0 {
			r.value = make([]byte, len(value))
			copy(r.value, value)
		}
	}
	// ts < r.timestamp: ignore (current value wins)
}

// Get returns the current value and its timestamp.
func (r *LWWRegister) Get() ([]byte, int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	val := make([]byte, len(r.value))
	copy(val, r.value)
	return val, r.timestamp
}

// Merge merges another LWWRegister into this one using Last-Writer-Wins semantics.
// After merge, this register holds the value with the highest timestamp.
func (r *LWWRegister) Merge(other *LWWRegister) {
	otherVal, otherTs := other.Get()
	r.Set(otherVal, otherTs)
}

// ==================== GCounter ====================

// GCounter is a Grow-only Counter CRDT.
// Each node maintains its own count. The total value is the sum of all node counts.
// Merge takes the maximum value for each node ID.
type GCounter struct {
	mu     sync.RWMutex
	id     string
	counts map[string]int64
}

// NewGCounter creates a new GCounter identified by the given node ID.
func NewGCounter(id string) *GCounter {
	return &GCounter{
		id:     id,
		counts: make(map[string]int64),
	}
}

// Increment adds delta to this node's counter.
func (g *GCounter) Increment(delta int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counts[g.id] += delta
}

// Value returns the total count by summing all node counters.
func (g *GCounter) Value() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var total int64
	for _, v := range g.counts {
		total += v
	}
	return total
}

// Merge merges another GCounter into this one using max-per-node semantics.
// For each node ID, the maximum count is retained.
func (g *GCounter) Merge(other *GCounter) {
	other.mu.RLock()
	defer other.mu.RUnlock()
	g.mu.Lock()
	defer g.mu.Unlock()

	for nodeID, count := range other.counts {
		if current, ok := g.counts[nodeID]; !ok || count > current {
			g.counts[nodeID] = count
		}
	}
}

// Counts returns a copy of the internal counts map (for serialization).
func (g *GCounter) Counts() map[string]int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]int64, len(g.counts))
	for k, v := range g.counts {
		result[k] = v
	}
	return result
}

// ID returns the node ID of this counter.
func (g *GCounter) ID() string {
	return g.id
}

// ==================== ORSet ====================

// ORSet is an Observed-Remove Set CRDT.
// Elements can be added and removed. Each add is tagged with a unique timestamp.
// A remove only affects elements that have been observed (added before the remove).
// Merge combines both add and remove maps using max-per-element semantics.
type ORSet struct {
	mu      sync.RWMutex
	id      string
	adds    map[string]int64 // element -> add timestamp
	removes map[string]int64 // element -> remove timestamp
}

// NewORSet creates a new ORSet identified by the given node ID.
func NewORSet(id string) *ORSet {
	return &ORSet{
		id:      id,
		adds:    make(map[string]int64),
		removes: make(map[string]int64),
	}
}

// Add adds an element to the set with the current unique timestamp.
// The element is considered present only if its add timestamp > remove timestamp.
func (s *ORSet) Add(element string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := nextTimestamp()
	currentAdd, hasAdd := s.adds[element]
	currentRemove, hasRemove := s.removes[element]

	if !hasAdd || ts > currentAdd {
		s.adds[element] = ts
	}
	// If element was previously removed and is being re-added,
	// ensure add timestamp is after remove timestamp
	_ = hasRemove // remove record stays (observed-remove semantics)
	if hasRemove && ts <= currentRemove {
		s.adds[element] = currentRemove + 1
	}
}

// Remove removes an element from the set.
// The remove is recorded with a timestamp. An element is considered removed
// only if its remove timestamp >= its add timestamp.
func (s *ORSet) Remove(element string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	addTs, hasAdd := s.adds[element]
	if !hasAdd {
		return // Can't remove what was never added
	}

	ts := nextTimestamp()
	// Ensure remove timestamp is at least the add timestamp
	if ts < addTs {
		ts = addTs
	}
	s.removes[element] = ts
}

// Contains returns true if the element is in the set (added and not removed).
func (s *ORSet) Contains(element string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addTs, hasAdd := s.adds[element]
	if !hasAdd {
		return false
	}

	removeTs, hasRemove := s.removes[element]
	if !hasRemove {
		return true
	}

	return addTs > removeTs
}

// Merge merges another ORSet into this one using observed-remove semantics.
// For each element, we keep the max of add timestamps and max of remove timestamps.
// An element is in the set if max(adds) > max(removes).
func (s *ORSet) Merge(other *ORSet) {
	other.mu.RLock()
	defer other.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()

	// Merge adds: take max timestamp for each element
	for elem, ts := range other.adds {
		if current, ok := s.adds[elem]; !ok || ts > current {
			s.adds[elem] = ts
		}
	}

	// Merge removes: take max timestamp for each element
	for elem, ts := range other.removes {
		if current, ok := s.removes[elem]; !ok || ts > current {
			s.removes[elem] = ts
		}
	}
}

// Elements returns all elements currently in the set.
func (s *ORSet) Elements() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []string
	for elem, addTs := range s.adds {
		removeTs, hasRemove := s.removes[elem]
		if !hasRemove || addTs > removeTs {
			result = append(result, elem)
		}
	}
	return result
}

// ==================== Helpers ====================

// timestampCounter is a monotonically increasing counter for generating unique timestamps.
var timestampCounter int64

// nextTimestamp returns the next unique timestamp.
func nextTimestamp() int64 {
	timestampCounter++
	return timestampCounter
}

// compareBytes compares two byte slices lexicographically.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareBytes(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}
