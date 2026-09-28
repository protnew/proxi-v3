package store

import (
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// SearchMessages with empty npub (global search)
// ---------------------------------------------------------------------------

func TestSearchMessages_GlobalSearch(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	msgs := []Message{
		{ID: "g1", From: "alice", To: "broadcast", Text: "golang tips", Timestamp: 1000},
		{ID: "g2", From: "bob", To: "carol", Text: "golang secrets", Timestamp: 1001},
		{ID: "g3", From: "dave", To: "eve", Text: "golang private", Timestamp: 1002},
		{ID: "g4", From: "alice", To: "broadcast", Text: "rust tips", Timestamp: 1003},
	}
	for _, m := range msgs {
		s.SaveMessage(m)
	}

	// Empty npub = global search — should find ALL messages matching "golang"
	results, err := s.SearchMessages("golang", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 global results for 'golang', got %d", len(results))
	}
}

func TestSearchMessages_Limit(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for i := 0; i < 20; i++ {
		s.SaveMessage(Message{
			ID:        fmt.Sprintf("lim%d", i),
			From:      "alice",
			To:        "broadcast",
			Text:      "test message",
			Timestamp: int64(i),
		})
	}

	results, err := s.SearchMessages("test", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results with limit=5, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// File Metadata CRUD
// ---------------------------------------------------------------------------

func TestFileMeta_CRUD(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	fm := FileMeta{
		ID:         "f1",
		Name:       "document.pdf",
		Size:       2048,
		Type:       "application/pdf",
		UploadedBy: "alice",
		CreatedAt:  1000,
	}
	if err := s.SaveFileMeta(fm); err != nil {
		t.Fatalf("SaveFileMeta: %v", err)
	}

	// Get
	got, err := s.GetFileMeta("f1")
	if err != nil {
		t.Fatalf("GetFileMeta: %v", err)
	}
	if got.Name != "document.pdf" || got.Size != 2048 || got.Type != "application/pdf" {
		t.Errorf("file meta mismatch: %+v", got)
	}
	if got.UploadedBy != "alice" {
		t.Errorf("expected uploadedBy=alice, got %q", got.UploadedBy)
	}

	// List
	files, err := s.ListFileMeta()
	if err != nil {
		t.Fatalf("ListFileMeta: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	// Delete
	if err := s.DeleteFileMeta("f1"); err != nil {
		t.Fatalf("DeleteFileMeta: %v", err)
	}

	// Verify deleted
	_, err = s.GetFileMeta("f1")
	if err == nil {
		t.Error("expected error for deleted file")
	}

	// Delete non-existent
	err = s.DeleteFileMeta("nonexistent")
	if err == nil {
		t.Error("expected error deleting non-existent file")
	}
}

func TestFileMeta_Multiple(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	files := []FileMeta{
		{ID: "f1", Name: "a.txt", Size: 100, Type: "text/plain", UploadedBy: "alice", CreatedAt: 1000},
		{ID: "f2", Name: "b.png", Size: 200, Type: "image/png", UploadedBy: "bob", CreatedAt: 2000},
		{ID: "f3", Name: "c.mp4", Size: 300, Type: "video/mp4", UploadedBy: "carol", CreatedAt: 3000},
	}
	for _, f := range files {
		s.SaveFileMeta(f)
	}

	list, err := s.ListFileMeta()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 files, got %d", len(list))
	}
	// Should be ordered by created_at DESC
	if list[0].ID != "f3" {
		t.Errorf("expected f3 first (newest), got %s", list[0].ID)
	}
}

func TestFileMeta_Update(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveFileMeta(FileMeta{ID: "f1", Name: "old.txt", Size: 10, Type: "text/plain", UploadedBy: "alice", CreatedAt: 1000})
	s.SaveFileMeta(FileMeta{ID: "f1", Name: "new.txt", Size: 20, Type: "text/plain", UploadedBy: "bob", CreatedAt: 2000})

	got, err := s.GetFileMeta("f1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "new.txt" {
		t.Errorf("expected updated name 'new.txt', got %q", got.Name)
	}
	if got.Size != 20 {
		t.Errorf("expected updated size 20, got %d", got.Size)
	}
}

// ---------------------------------------------------------------------------
// SearchMessagesFTS additional tests
// ---------------------------------------------------------------------------

func TestSearchMessagesFTS_WhitespaceOnly(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveMessage(Message{ID: "m1", From: "alice", To: "broadcast", Text: "hello", Timestamp: 1000})

	results, err := s.SearchMessagesFTS("   ", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("whitespace-only query should return nil")
	}
}

func TestSearchMessagesFTS_SpecialChars(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveMessage(Message{ID: "m1", From: "alice", To: "broadcast", Text: "hello world", Timestamp: 1000})
	s.SaveMessage(Message{ID: "m2", From: "bob", To: "broadcast", Text: "world peace", Timestamp: 1001})

	// Query with special FTS characters should be sanitized
	// After sanitization, "hello" should still work
	results, err := s.SearchMessagesFTS(`hello`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 1 {
		t.Error("expected at least 1 result for 'hello'")
	}

	// Complex query with special chars that get stripped
	// The FTS fallback passes the original query to LIKE search
	results, err = s.SearchMessagesFTS(`hello world`, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Should match "hello world"
	if len(results) < 1 {
		t.Error("expected at least 1 result for 'hello world'")
	}
}

// ---------------------------------------------------------------------------
// Nostr Events: DeleteOldNostrEvents
// ---------------------------------------------------------------------------

func TestDeleteOldNostrEvents(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	events := []NostrEvent{
		{ID: "old1", PubKey: "pk1", Kind: 1, Content: "old event 1", CreatedAt: 100},
		{ID: "old2", PubKey: "pk1", Kind: 1, Content: "old event 2", CreatedAt: 200},
		{ID: "new1", PubKey: "pk1", Kind: 1, Content: "new event", CreatedAt: 300},
	}
	for _, e := range events {
		s.SaveNostrEvent(e)
	}

	deleted, err := s.DeleteOldNostrEvents(250)
	if err != nil {
		t.Fatalf("DeleteOldNostrEvents: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted, got %d", deleted)
	}

	remaining, _ := s.GetNostrEvents(NostrEventFilter{})
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining event, got %d", len(remaining))
	}
	if remaining[0].ID != "new1" {
		t.Errorf("expected remaining 'new1', got %q", remaining[0].ID)
	}
}

func TestDeleteOldNostrEvents_NoneOld(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveNostrEvent(NostrEvent{ID: "e1", PubKey: "pk1", Kind: 1, Content: "future", CreatedAt: 1000})

	deleted, err := s.DeleteOldNostrEvents(500)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}

// ---------------------------------------------------------------------------
// Scheduled Messages
// ---------------------------------------------------------------------------

func TestScheduledMessages(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Now().Unix()

	msgs := []ScheduledMessage{
		{ID: "s1", Sender: "alice", Recipient: "bob", Text: "past due", SendAt: now - 100, Status: "pending", CreatedAt: now - 200},
		{ID: "s2", Sender: "alice", Recipient: "bob", Text: "future", SendAt: now + 10000, Status: "pending", CreatedAt: now},
		{ID: "s3", Sender: "bob", Recipient: "broadcast", Text: "already sent", SendAt: now - 50, Status: "sent", CreatedAt: now - 100},
	}
	for _, m := range msgs {
		if err := s.SaveScheduledMessage(m); err != nil {
			t.Fatalf("SaveScheduledMessage %s: %v", m.ID, err)
		}
	}

	// GetPendingScheduled should return only pending with send_at <= now
	pending, err := s.GetPendingScheduled()
	if err != nil {
		t.Fatalf("GetPendingScheduled: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending (s1), got %d", len(pending))
	}
	if pending[0].ID != "s1" {
		t.Errorf("expected pending s1, got %s", pending[0].ID)
	}

	// Mark as sent
	if err := s.MarkScheduledSent("s1"); err != nil {
		t.Fatalf("MarkScheduledSent: %v", err)
	}

	// Now no pending should remain
	pending, _ = s.GetPendingScheduled()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after marking sent, got %d", len(pending))
	}
}

// ---------------------------------------------------------------------------
// Dead Man's Switch
// ---------------------------------------------------------------------------

func TestDeadMansSwitch(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Create the dead_mans_switch table (not yet in versioned migrations)
	s.DB().Exec(`CREATE TABLE IF NOT EXISTS dead_mans_switch (
		id TEXT PRIMARY KEY,
		user_npub TEXT NOT NULL,
		message_text TEXT NOT NULL,
		recipient TEXT NOT NULL DEFAULT 'broadcast',
		interval_days INTEGER NOT NULL DEFAULT 7,
		last_check_in INTEGER NOT NULL DEFAULT 0,
		triggered INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL DEFAULT 0
	)`)

	now := time.Now().Unix()

	dms := DeadMansSwitch{
		ID:           "dms1",
		UserNpub:     "alice",
		MessageText:  "Goodbye world",
		Recipient:    "broadcast",
		IntervalDays: 7,
		LastCheckIn:  now - 8*86400, // 8 days ago
		Triggered:    false,
		CreatedAt:    now - 30*86400,
	}
	if err := s.SaveDeadMansSwitch(dms); err != nil {
		t.Fatalf("SaveDeadMansSwitch: %v", err)
	}

	// GetExpiredSwitches should find it
	expired, err := s.GetExpiredSwitches()
	if err != nil {
		t.Fatalf("GetExpiredSwitches: %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired switch, got %d", len(expired))
	}
	if expired[0].ID != "dms1" {
		t.Errorf("expected dms1, got %s", expired[0].ID)
	}

	// Check in should prevent expiration
	if err := s.CheckInDeadMansSwitch("alice"); err != nil {
		t.Fatalf("CheckInDeadMansSwitch: %v", err)
	}

	expired, _ = s.GetExpiredSwitches()
	if len(expired) != 0 {
		t.Errorf("expected 0 expired after check-in, got %d", len(expired))
	}

	// Mark triggered
	if err := s.MarkSwitchTriggered("dms1"); err != nil {
		t.Fatalf("MarkSwitchTriggered: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Group Members
// ---------------------------------------------------------------------------

func TestGroupMembers_CRUD(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Create the group_members table (not yet in versioned migrations)
	s.DB().Exec(`CREATE TABLE IF NOT EXISTS group_members (
		group_id TEXT NOT NULL,
		user_npub TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'member',
		joined_at INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (group_id, user_npub)
	)`)

	now := time.Now().Unix()

	// Save members
	members := []GroupMember{
		{GroupID: "g1", UserNpub: "alice", Role: "admin", JoinedAt: now},
		{GroupID: "g1", UserNpub: "bob", Role: "member", JoinedAt: now},
		{GroupID: "g1", UserNpub: "carol", Role: "moderator", JoinedAt: now},
	}
	for _, m := range members {
		if err := s.SaveGroupMember(m); err != nil {
			t.Fatalf("SaveGroupMember: %v", err)
		}
	}

	// Get members
	got, err := s.GetGroupMembers("g1")
	if err != nil {
		t.Fatalf("GetGroupMembers: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 members, got %d", len(got))
	}

	// IsGroupAdmin
	isAdmin, err := s.IsGroupAdmin("g1", "alice")
	if err != nil {
		t.Fatalf("IsGroupAdmin: %v", err)
	}
	if !isAdmin {
		t.Error("alice should be admin")
	}

	isAdmin, _ = s.IsGroupAdmin("g1", "bob")
	if isAdmin {
		t.Error("bob should not be admin")
	}

	// Update role
	if err := s.UpdateGroupMemberRole("g1", "bob", "admin"); err != nil {
		t.Fatalf("UpdateGroupMemberRole: %v", err)
	}
	isAdmin, _ = s.IsGroupAdmin("g1", "bob")
	if !isAdmin {
		t.Error("bob should now be admin")
	}

	// Remove member
	if err := s.RemoveGroupMember("g1", "carol"); err != nil {
		t.Fatalf("RemoveGroupMember: %v", err)
	}
	got, _ = s.GetGroupMembers("g1")
	if len(got) != 2 {
		t.Fatalf("expected 2 members after removal, got %d", len(got))
	}

	// Remove non-existent
	err = s.RemoveGroupMember("g1", "nonexistent")
	if err == nil {
		t.Error("expected error removing non-existent member")
	}
}

func TestGroupMembers_EmptyGroup(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Create the group_members table (not yet in versioned migrations)
	s.DB().Exec(`CREATE TABLE IF NOT EXISTS group_members (
		group_id TEXT NOT NULL,
		user_npub TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'member',
		joined_at INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (group_id, user_npub)
	)`)

	got, err := s.GetGroupMembers("nonexistent")
	if err != nil {
		t.Fatalf("GetGroupMembers on empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 members for nonexistent group, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// EditMessage
// ---------------------------------------------------------------------------

func TestEditMessage(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveMessage(Message{ID: "m1", From: "alice", To: "broadcast", Text: "original", Timestamp: 1000})

	if err := s.EditMessage("m1", "edited text", "alice"); err != nil {
		t.Fatalf("EditMessage: %v", err)
	}

	got, err := s.GetMessageByID("m1")
	if err != nil {
		t.Fatalf("GetMessageByID: %v", err)
	}
	if got.Text != "edited text" {
		t.Errorf("expected 'edited text', got %q", got.Text)
	}
}

func TestEditMessage_NotFound(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.EditMessage("nonexistent", "new text", "user1")
	if err == nil {
		t.Error("expected error editing non-existent message")
	}
}

func TestDeleteMessage_NotFound(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.DeleteMessage("nonexistent", "user1")
	if err == nil {
		t.Error("expected error deleting non-existent message")
	}
}

// ---------------------------------------------------------------------------
// SubscribeChannel
// ---------------------------------------------------------------------------

func TestSubscribeChannel(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveChannel(Channel{ID: "ch1", Name: "general", Creator: "alice", CreatedAt: 1000})

	if err := s.SubscribeChannel("ch1"); err != nil {
		t.Fatalf("SubscribeChannel: %v", err)
	}

	channels, _ := s.GetChannels()
	if channels[0].Subscribers != 1 {
		t.Errorf("expected 1 subscriber, got %d", channels[0].Subscribers)
	}

	// Subscribe again
	s.SubscribeChannel("ch1")
	channels, _ = s.GetChannels()
	if channels[0].Subscribers != 2 {
		t.Errorf("expected 2 subscribers, got %d", channels[0].Subscribers)
	}
}

func TestSubscribeChannel_NotFound(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.SubscribeChannel("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent channel")
	}
}

// ---------------------------------------------------------------------------
// CleanExpiredMessages
// ---------------------------------------------------------------------------

func TestCleanExpiredMessages_NoExpired(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveMessage(Message{ID: "m1", From: "alice", To: "broadcast", Text: "permanent", Timestamp: time.Now().Unix(), TTL: 0})

	n, err := s.CleanExpiredMessages()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 expired, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// GetProfile_NotFound
// ---------------------------------------------------------------------------

func TestGetProfile_NotFound(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, err = s.GetProfile("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

// ---------------------------------------------------------------------------
// SearchProfiles_EmptyResult
// ---------------------------------------------------------------------------

func TestSearchProfiles_EmptyResult(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	results, err := s.SearchProfiles("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// GetMessageByID
// ---------------------------------------------------------------------------

func TestGetMessageByID_NotFound(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, err = s.GetMessageByID("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent message")
	}
}

func TestGetMessageByID_Found(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveMessage(Message{ID: "m1", From: "alice", To: "bob", Text: "hello", Timestamp: 1000, Encrypted: true})
	got, err := s.GetMessageByID("m1")
	if err != nil {
		t.Fatal(err)
	}
	if got.From != "alice" || !got.Encrypted {
		t.Errorf("message mismatch: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// DB() accessor
// ---------------------------------------------------------------------------

func TestStore_DB(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if s.DB() == nil {
		t.Error("DB() should return non-nil")
	}
}

// ---------------------------------------------------------------------------
// SaveProfile update
// ---------------------------------------------------------------------------

func TestSaveProfile_Update(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveProfile("npub1", "Alice", "", "Hello")
	s.SaveProfile("npub1", "Alice Updated", "https://new.avatar/url", "New bio")

	profile, err := s.GetProfile("npub1")
	if err != nil {
		t.Fatal(err)
	}
	if profile["displayName"] != "Alice Updated" {
		t.Errorf("expected 'Alice Updated', got %v", profile["displayName"])
	}
	if profile["bio"] != "New bio" {
		t.Errorf("expected 'New bio', got %v", profile["bio"])
	}
}
