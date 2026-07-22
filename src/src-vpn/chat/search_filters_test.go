package chat

import (
	"testing"
)

// helper to build test messages quickly.
func mkMsg(id, from, to, text string, ts int64) Message {
	return Message{ID: id, From: from, To: to, Text: text, Ts: ts}
}

// ---------------------------------------------------------------------------
// TestSearchByText — basic query matching (case-insensitive)
// ---------------------------------------------------------------------------

func TestSearchByText(t *testing.T) {
	t.Parallel()

	se := NewSearchEngine()
	se.Add(
		mkMsg("1", "alice", "chan1", "Hello World", 1000),
		mkMsg("2", "bob", "chan1", "hello there", 1001),
		mkMsg("3", "carol", "chan1", "Goodbye", 1002),
	)

	// Case-insensitive search.
	results, err := se.SearchWithFilters("hello", SearchFilter{})
	if err != nil {
		t.Fatalf("SearchWithFilters error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'hello', got %d", len(results))
	}

	// Empty query returns all.
	results, _ = se.SearchWithFilters("", SearchFilter{})
	if len(results) != 3 {
		t.Fatalf("expected 3 results for empty query, got %d", len(results))
	}

	// No match.
	results, _ = se.SearchWithFilters("nonexistent", SearchFilter{})
	if len(results) != 0 {
		t.Fatalf("expected 0 results for 'nonexistent', got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// TestSearchFilterByDate — date range filtering
// ---------------------------------------------------------------------------

func TestSearchFilterByDate(t *testing.T) {
	t.Parallel()

	se := NewSearchEngine()
	se.Add(
		mkMsg("1", "a", "ch", "msg", 1000),
		mkMsg("2", "a", "ch", "msg", 2000),
		mkMsg("3", "a", "ch", "msg", 3000),
		mkMsg("4", "a", "ch", "msg", 4000),
	)

	// DateFrom only.
	results, _ := se.SearchWithFilters("", SearchFilter{DateFrom: 2500})
	if len(results) != 2 {
		t.Fatalf("DateFrom=2500 expected 2 results, got %d", len(results))
	}

	// DateTo only: ts 1000,2000 <= 2500.
	results, _ = se.SearchWithFilters("", SearchFilter{DateTo: 2500})
	if len(results) != 2 {
		t.Fatalf("DateTo=2500 expected 2 results, got %d", len(results))
	}

	// Range.
	results, _ = se.SearchWithFilters("", SearchFilter{DateFrom: 1500, DateTo: 3500})
	if len(results) != 2 {
		t.Fatalf("range expected 2 results, got %d", len(results))
	}
	if results[0].Ts != 2000 || results[1].Ts != 3000 {
		t.Fatalf("range results wrong: %d, %d", results[0].Ts, results[1].Ts)
	}
}

// ---------------------------------------------------------------------------
// TestSearchFilterByUser — filter by sender
// ---------------------------------------------------------------------------

func TestSearchFilterByUser(t *testing.T) {
	t.Parallel()

	se := NewSearchEngine()
	se.Add(
		mkMsg("1", "alice", "ch", "hi", 1000),
		mkMsg("2", "bob", "ch", "hi", 1001),
		mkMsg("3", "alice", "ch", "yo", 1002),
	)

	results, _ := se.SearchWithFilters("", SearchFilter{UserID: "alice"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results from alice, got %d", len(results))
	}
	for _, m := range results {
		if m.From != "alice" {
			t.Fatalf("unexpected sender %s", m.From)
		}
	}

	// Combined with query.
	results, _ = se.SearchWithFilters("yo", SearchFilter{UserID: "alice"})
	if len(results) != 1 || results[0].ID != "3" {
		t.Fatalf("expected [3], got %v", results)
	}
}

// ---------------------------------------------------------------------------
// TestSearchFilterByChannel — filter by channel/recipient
// ---------------------------------------------------------------------------

func TestSearchFilterByChannel(t *testing.T) {
	t.Parallel()

	se := NewSearchEngine()
	se.Add(
		mkMsg("1", "a", "chan-X", "hello", 1000),
		mkMsg("2", "a", "chan-Y", "hello", 1001),
		mkMsg("3", "a", "chan-X", "hello", 1002),
	)

	results, _ := se.SearchWithFilters("hello", SearchFilter{ChannelID: "chan-X"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results in chan-X, got %d", len(results))
	}
	for _, m := range results {
		if m.To != "chan-X" {
			t.Fatalf("unexpected channel %s", m.To)
		}
	}
}

// ---------------------------------------------------------------------------
// TestSearchFilterByAttachments — filter by attachment presence
// ---------------------------------------------------------------------------

func TestSearchFilterByAttachments(t *testing.T) {
	t.Parallel()

	se := NewSearchEngine()
	se.Add(
		mkMsg("plain1", "a", "ch", "text only", 1000),
		Message{ID: "voice1", From: "a", To: "ch", Text: "voice msg", Ts: 1001, VoiceDuration: 5, VoiceData: "base64data"},
		Message{ID: "fwd1", From: "a", To: "ch", Text: "forwarded", Ts: 1002, ForwardedFrom: "orig-npub"},
		mkMsg("plain2", "a", "ch", "more text", 1003),
	)

	hasAtt := true
	results, _ := se.SearchWithFilters("", SearchFilter{HasAttachments: &hasAtt})
	if len(results) != 2 {
		t.Fatalf("expected 2 messages with attachments, got %d", len(results))
	}

	noAtt := false
	results, _ = se.SearchWithFilters("", SearchFilter{HasAttachments: &noAtt})
	if len(results) != 2 {
		t.Fatalf("expected 2 messages without attachments, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// TestSearchFilterCombined — all filters combined
// ---------------------------------------------------------------------------

func TestSearchFilterCombined(t *testing.T) {
	t.Parallel()

	se := NewSearchEngine()
	se.Add(
		mkMsg("1", "alice", "chan-A", "important message", 1000),
		mkMsg("2", "bob", "chan-A", "important message", 1500),
		mkMsg("3", "alice", "chan-B", "important message", 2000),
		mkMsg("4", "alice", "chan-A", "important message", 3000),
		mkMsg("5", "bob", "chan-A", "unrelated", 2500),
	)

	results, err := se.SearchWithFilters("important", SearchFilter{
		DateFrom:  1500,
		DateTo:    3500,
		UserID:    "alice",
		ChannelID: "chan-A",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Only msg 4 matches: alice, chan-A, ts between 1500-3500, text contains "important".
	if len(results) != 1 {
		t.Fatalf("expected 1 combined result, got %d", len(results))
	}
	if results[0].ID != "4" {
		t.Fatalf("expected result ID 4, got %s", results[0].ID)
	}
}
