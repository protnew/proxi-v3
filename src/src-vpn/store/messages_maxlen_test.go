package store

import (
	"strings"
	"testing"
)

func TestSaveMessageRejectsOversize(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	long := strings.Repeat("x", MaxMessageLen+1)
	if err := s.SaveMessage(Message{ID: "t1", From: "a", To: "b", Text: long, Timestamp: 1}); err == nil {
		t.Fatal("expected error for oversize message")
	}
	if err := s.SaveMessage(Message{ID: "t2", From: "a", To: "b", Text: "hi", Timestamp: 1}); err != nil {
		t.Fatalf("short message should save: %v", err)
	}
}
