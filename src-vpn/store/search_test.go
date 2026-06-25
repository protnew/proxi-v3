package store

import (
	"testing"
)

func TestSearchMessagesFTS_Fallback(t *testing.T) {
	db, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SaveMessage(Message{ID: "m1", From: "alice", To: "bob", Text: "hello world", Timestamp: 1000})
	db.SaveMessage(Message{ID: "m2", From: "bob", To: "alice", Text: "goodbye moon", Timestamp: 1001})
	db.SaveMessage(Message{ID: "m3", From: "carol", To: "bob", Text: "hello again", Timestamp: 1002})

	results, err := db.SearchMessagesFTS("hello", 10)
	if err != nil {
		t.Fatal(err)
	}
	// Without FTS5 compiled in, falls back to LIKE search
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
}

func TestSearchMessagesFTS_Empty(t *testing.T) {
	db, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	results, err := db.SearchMessagesFTS("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("empty query should return nil")
	}
}

func TestSanitizeFTSQuery(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"hello world", "hello* AND world*"},
		{"", ""},
		{"   ", ""},
		// Quotes are stripped before tokenization, so "testquery" becomes one token
		{`test"query`, "testquery*"},
		{"test*", "test*"},
	}
	for _, tt := range tests {
		got := sanitizeFTSQuery(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFTSQuery(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSearchChannelsFTS(t *testing.T) {
	db, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Use correct column name: subscribers (not subscriber_count)
	db.DB().Exec("INSERT INTO channels (id, name, description, creator, subscribers, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"c1", "crypto news", "All about blockchain", "alice", 100, 1000)
	db.DB().Exec("INSERT INTO channels (id, name, description, creator, subscribers, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"c2", "tech talk", "Technology discussion", "bob", 200, 1001)

	results, err := db.SearchChannelsFTS("crypto", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "crypto news" {
		t.Errorf("expected 'crypto news', got '%s'", results[0].Name)
	}
}
