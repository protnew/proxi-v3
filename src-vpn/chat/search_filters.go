package chat

import (
	"strings"
	"sync"
)

// SearchFilter specifies optional criteria for narrowing a message search.
// Zero-value fields mean "no constraint on this dimension".
type SearchFilter struct {
	DateFrom       int64 // inclusive lower bound on Ts (unix seconds, 0 = no bound)
	DateTo         int64 // inclusive upper bound on Ts (unix seconds, 0 = no bound)
	UserID         string // restrict to messages From this user ("" = any sender)
	ChannelID      string // restrict to messages To this channel/recipient ("" = any)
	HasAttachments *bool  // nil = no filter; true = only messages with attachments; false = only without
}

// Match reports whether a message satisfies the filter.
func (f SearchFilter) Match(m *Message) bool {
	if f.DateFrom != 0 && m.Ts < f.DateFrom {
		return false
	}
	if f.DateTo != 0 && m.Ts > f.DateTo {
		return false
	}
	if f.UserID != "" && m.From != f.UserID {
		return false
	}
	if f.ChannelID != "" && m.To != f.ChannelID {
		return false
	}
	if f.HasAttachments != nil {
		has := MessageHasAttachments(m)
		if *f.HasAttachments != has {
			return false
		}
	}
	return true
}

// MessageHasAttachments reports whether a message carries an attachment.
// In the current Message model this includes voice data, voice duration, or
// forwarded content.
func MessageHasAttachments(m *Message) bool {
	return m.VoiceData != "" || m.VoiceDuration > 0 || m.ForwardedFrom != ""
}

// SearchEngine is an in-memory, concurrently-safe store of messages that
// supports filtered full-text search over message bodies.
type SearchEngine struct {
	mu       sync.RWMutex
	messages []Message
}

// NewSearchEngine creates a new empty SearchEngine.
func NewSearchEngine() *SearchEngine {
	return &SearchEngine{}
}

// Add inserts one or more messages into the search index.
func (se *SearchEngine) Add(msgs ...Message) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.messages = append(se.messages, msgs...)
}

// Size returns the number of indexed messages.
func (se *SearchEngine) Size() int {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return len(se.messages)
}

// SearchWithFilters returns all indexed messages whose Text contains the
// query (case-insensitive substring) and which satisfy the given filter.
// An empty query matches all text (acts as a pure filter).
// Results are returned in insertion order.
func (se *SearchEngine) SearchWithFilters(query string, filter SearchFilter) ([]Message, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	q := strings.ToLower(query)
	var results []Message
	for i := range se.messages {
		m := &se.messages[i]
		if q != "" && !strings.Contains(strings.ToLower(m.Text), q) {
			continue
		}
		if !filter.Match(m) {
			continue
		}
		results = append(results, *m)
	}
	return results, nil
}
