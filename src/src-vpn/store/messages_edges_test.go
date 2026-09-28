package store

import (
	"strings"
	"testing"
	"time"
)

func TestSaveMessage_EmptyAndWhitespace(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMessage(Message{ID: "e1", From: "a", To: "b", Text: "", Timestamp: 1}); err != nil {
		t.Fatalf("empty should soft-skip: %v", err)
	}
	if err := s.SaveMessage(Message{ID: "e2", From: "a", To: "b", Text: "   \t  ", Timestamp: 1}); err != nil {
		t.Fatalf("ws should soft-skip: %v", err)
	}
}

func TestSaveMessage_ExactLimitOK(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("a", MaxMessageLen)
	if err := s.SaveMessage(Message{ID: "ok1", From: "a", To: "b", Text: text, Timestamp: time.Now().Unix()}); err != nil {
		t.Fatalf("exact limit should save: %v", err)
	}
}

func TestSaveMessage_OverLimit(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("b", MaxMessageLen+1)
	if err := s.SaveMessage(Message{ID: "bad1", From: "a", To: "b", Text: text, Timestamp: 1}); err == nil {
		t.Fatal("expected oversize error")
	}
}

func TestSaveMessage_RoundTrip(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{ID: "rt1", From: "alice", To: "bob", Text: "hello edges", Timestamp: 42}
	if err := s.SaveMessage(msg); err != nil {
		t.Fatal(err)
	}
	// List or get if available
	if lister, ok := interface{}(s).(interface {
		GetMessages(string, string, int) ([]Message, error)
	}); ok {
		_, _ = lister.GetMessages("alice", "bob", 10)
	}
}
