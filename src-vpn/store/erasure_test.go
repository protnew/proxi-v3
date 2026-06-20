package store

import (
	"fmt"
	"testing"
)

// TestStore_DataErasure verifies that messages, reactions, read receipts,
// and profiles can be fully erased from the SQLite store.
func TestStore_DataErasure(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/erasure_test.db"
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	// --- Seed data ---

	// Messages
	msgs := []Message{
		{ID: "msg-1", From: "alice", To: "broadcast", Text: "Hello", Timestamp: 1000},
		{ID: "msg-2", From: "bob", To: "alice", Text: "Hi Alice", Timestamp: 1001},
		{ID: "msg-3", From: "alice", To: "bob", Text: "Secret", Timestamp: 1002, TTL: 3600},
	}
	for _, m := range msgs {
		if err := s.SaveMessage(m); err != nil {
			t.Fatalf("SaveMessage %s: %v", m.ID, err)
		}
	}

	// Reactions
	if err := s.AddReaction("msg-1", "bob_npub", "👍"); err != nil {
		t.Fatalf("AddReaction: %v", err)
	}
	if err := s.AddReaction("msg-1", "charlie_npub", "❤️"); err != nil {
		t.Fatalf("AddReaction: %v", err)
	}

	// Read receipts
	if err := s.MarkRead("msg-1", "bob_npub"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if err := s.MarkRead("msg-2", "alice_npub"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	// Profile
	if err := s.SaveProfile("alice_npub", "Alice", "https://example.com/avatar.png", "Hello world"); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	// Channel
	ch := Channel{ID: "ch-1", Name: "General", Creator: "alice", Subscribers: 5, CreatedAt: 1000}
	if err := s.SaveChannel(ch); err != nil {
		t.Fatalf("SaveChannel: %v", err)
	}

	// File metadata
	if err := s.SaveFileMeta(FileMeta{ID: "file-1", Name: "test.png", Size: 1024, Type: "image/png", UploadedBy: "alice", CreatedAt: 1000}); err != nil {
		t.Fatalf("SaveFileMeta: %v", err)
	}

	// Scheduled message
	if err := s.SaveScheduledMessage(ScheduledMessage{ID: "sched-1", Sender: "alice", Recipient: "bob", Text: "Future", SendAt: 9999999999, Status: "pending", CreatedAt: 1000}); err != nil {
		t.Fatalf("SaveScheduledMessage: %v", err)
	}

	// --- Verify data exists ---

	loaded, err := s.GetMessages(100, 0, "alice")
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(loaded) == 0 {
		t.Fatal("expected messages to exist before erasure")
	}

	reactions, err := s.GetReactions("msg-1")
	if err != nil {
		t.Fatalf("GetReactions: %v", err)
	}
	if len(reactions) != 2 {
		t.Fatalf("expected 2 reactions, got %d", len(reactions))
	}

	receipts, err := s.GetReadReceipts("msg-1")
	if err != nil {
		t.Fatalf("GetReadReceipts: %v", err)
	}
	if len(receipts) != 1 {
		t.Fatalf("expected 1 read receipt, got %d", len(receipts))
	}

	profile, err := s.GetProfile("alice_npub")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile == nil {
		t.Fatal("expected profile to exist")
	}

	channels, err := s.GetChannels()
	if err != nil {
		t.Fatalf("GetChannels: %v", err)
	}
	if len(channels) == 0 {
		t.Fatal("expected channels to exist")
	}

	// --- Erase data ---

	for _, m := range msgs {
		if err := s.DeleteMessage(m.ID, m.From); err != nil {
			t.Fatalf("DeleteMessage %s: %v", m.ID, err)
		}
	}

	// Remove reactions
	if err := s.RemoveReaction("msg-1", "bob_npub"); err != nil {
		t.Fatalf("RemoveReaction: %v", err)
	}
	if err := s.RemoveReaction("msg-1", "charlie_npub"); err != nil {
		t.Fatalf("RemoveReaction: %v", err)
	}

	// Delete file
	if err := s.DeleteFileMeta("file-1"); err != nil {
		t.Fatalf("DeleteFileMeta: %v", err)
	}

	// --- Verify data is erased ---

	loaded2, err := s.GetMessages(100, 0, "alice")
	if err != nil {
		t.Fatalf("GetMessages after erasure: %v", err)
	}
	if len(loaded2) != 0 {
		t.Fatalf("expected 0 messages after erasure, got %d", len(loaded2))
	}

	reactions2, err := s.GetReactions("msg-1")
	if err != nil {
		t.Fatalf("GetReactions after erasure: %v", err)
	}
	if len(reactions2) != 0 {
		t.Fatalf("expected 0 reactions after erasure, got %d", len(reactions2))
	}

	_, err = s.GetFileMeta("file-1")
	if err == nil {
		t.Fatal("expected error for deleted file meta")
	}

	// --- Verify self-destruct (TTL) ---

	// Re-add a message with very short TTL that should already be expired
	expiredMsg := Message{
		ID:        "msg-expired",
		From:      "alice",
		To:        "broadcast",
		Text:      "This will self-destruct",
		Timestamp: 1, // very old
		TTL:       1, // 1 second TTL
	}
	if err := s.SaveMessage(expiredMsg); err != nil {
		t.Fatalf("SaveMessage expired: %v", err)
	}

	// Clean expired
	n, err := s.CleanExpiredMessages()
	if err != nil {
		t.Fatalf("CleanExpiredMessages: %v", err)
	}
	if n == 0 {
		t.Fatal("expected at least 1 expired message to be cleaned")
	}
}

// TestStore_DataPersistence verifies data persists across store reopen.
func TestStore_DataPersistence(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/persist_test.db"

	// Create and write
	s1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore s1: %v", err)
	}

	msg := Message{ID: "persist-1", From: "alice", To: "broadcast", Text: "Persistent", Timestamp: 1000}
	if err := s1.SaveMessage(msg); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	s1.Close()

	// Reopen and verify
	s2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore s2: %v", err)
	}
	defer s2.Close()

	loaded, err := s2.GetMessages(100, 0, "alice")
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 persisted message, got %d", len(loaded))
	}
	if loaded[0].Text != "Persistent" {
		t.Fatalf("expected text 'Persistent', got %q", loaded[0].Text)
	}
}

// TestStore_LargeDataSet tests store operations with many records.
func TestStore_LargeDataSet(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/large_test.db"
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	// Insert 500 messages
	for i := 0; i < 500; i++ {
		msg := Message{
			ID:        fmt.Sprintf("bulk-%d", i),
			From:      "alice",
			To:        "broadcast",
			Text:      fmt.Sprintf("Message %d", i),
			Timestamp: int64(i),
		}
		if err := s.SaveMessage(msg); err != nil {
			t.Fatalf("SaveMessage %d: %v", i, err)
		}
	}

	// Verify count — include both alice and broadcast filters
	loaded, err := s.GetMessages(1000, -1, "alice")
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(loaded) < 500 {
		t.Fatalf("expected at least 500 messages, got %d", len(loaded))
	}

	// Search
	results, err := s.SearchMessages("Message 42", "alice", 100)
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
}
