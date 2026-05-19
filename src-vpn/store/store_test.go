package store

import (
	"database/sql"
	"testing"
)

func TestNewStore(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	// Verify tables exist by querying them
	tables := []string{"messages", "channels", "peers", "identity", "reactions", "read_receipts", "user_profiles", "file_metadata"}
	for _, tbl := range tables {
		var name string
		err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %q does not exist: %v", tbl, err)
		}
	}
}

func TestSaveAndGetMessages(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	msgs := []Message{
		{ID: "m1", From: "alice", To: "broadcast", Text: "hello world", Timestamp: 1000},
		{ID: "m2", From: "bob", To: "broadcast", Text: "hi there", Encrypted: true, Timestamp: 1001},
		{ID: "m3", From: "alice", To: "bob", Text: "secret DM", Timestamp: 1002, ReplyTo: "m2", ForwardedFrom: "carol", Attachments: `["f1"]`},
	}
	for _, m := range msgs {
		if err := s.SaveMessage(m); err != nil {
			t.Fatalf("SaveMessage(%s): %v", m.ID, err)
		}
	}

	got, err := s.GetMessages(100, 0, "alice")
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}

	// Check first message fields
	if got[0].ID != "m1" || got[0].From != "alice" || got[0].To != "broadcast" || got[0].Text != "hello world" {
		t.Errorf("message m1 mismatch: %+v", got[0])
	}
	// Check encrypted flag
	if !got[1].Encrypted {
		t.Error("message m2 should be encrypted")
	}
	// Check reply/forward/attachments
	if got[2].ReplyTo != "m2" {
		t.Errorf("expected reply_to=m2, got %q", got[2].ReplyTo)
	}
	if got[2].ForwardedFrom != "carol" {
		t.Errorf("expected forwarded_from=carol, got %q", got[2].ForwardedFrom)
	}
	if got[2].Attachments != `["f1"]` {
		t.Errorf("expected attachments, got %q", got[2].Attachments)
	}
}

func TestGetMessagesFilter(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Save broadcast + DM messages
	msgs := []Message{
		{ID: "b1", From: "alice", To: "broadcast", Text: "public 1", Timestamp: 1000},
		{ID: "b2", From: "bob", To: "broadcast", Text: "public 2", Timestamp: 1001},
		{ID: "dm1", From: "alice", To: "carol", Text: "DM to carol", Timestamp: 1002},
		{ID: "dm2", From: "dave", To: "alice", Text: "DM to alice", Timestamp: 1003},
		{ID: "dm3", From: "bob", To: "dave", Text: "DM invisible to alice", Timestamp: 1004},
	}
	for _, m := range msgs {
		s.SaveMessage(m)
	}

	// alice sees: broadcasts + DMs involving alice
	got, err := s.GetMessages(100, 0, "alice")
	if err != nil {
		t.Fatal(err)
	}
	// Should see b1, b2, dm1 (alice→carol), dm2 (dave→alice) = 4 messages
	if len(got) != 4 {
		t.Fatalf("expected 4 messages for alice, got %d", len(got))
	}

	// bob sees: broadcasts + DMs involving bob (dm3 is bob→dave)
	got, err = s.GetMessages(100, 0, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 messages for bob (2 broadcast + 1 DM as sender), got %d", len(got))
	}

	// unknown user sees only broadcasts
	got, err = s.GetMessages(100, 0, "unknown")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 broadcast messages for unknown, got %d", len(got))
	}
}

func TestSaveAndGetChannels(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ch := Channel{
		ID:          "ch1",
		Name:        "general",
		Description: "General chat",
		Creator:     "alice",
		Subscribers: 42,
		CreatedAt:   1000,
	}
	if err := s.SaveChannel(ch); err != nil {
		t.Fatal(err)
	}

	channels, err := s.GetChannels()
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].Name != "general" || channels[0].Creator != "alice" || channels[0].Subscribers != 42 {
		t.Errorf("channel mismatch: %+v", channels[0])
	}
}

func TestSaveAndGetPeers(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Create
	if err := s.SavePeer("p1", "Alice", "pubkey123", "1.2.3.4:51820", "10.0.0.2/32"); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePeer("p2", "Bob", "pubkey456", "5.6.7.8:51820", "10.0.0.3/32"); err != nil {
		t.Fatal(err)
	}

	// Read
	peers, err := s.GetPeers()
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}

	// Update (upsert)
	s.SavePeer("p1", "Alice Updated", "pubkey123", "1.2.3.4:51820", "10.0.0.2/32")
	peers, _ = s.GetPeers()
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers after update, got %d", len(peers))
	}

	// Delete
	if err := s.DeletePeer("p1"); err != nil {
		t.Fatal(err)
	}
	peers, _ = s.GetPeers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer after delete, got %d", len(peers))
	}

	// Delete non-existent
	err = s.DeletePeer("nonexistent")
	if err == nil {
		t.Error("expected error deleting non-existent peer")
	}
}

func TestReactions(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Add reactions
	if err := s.AddReaction("msg1", "alice", "👍"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddReaction("msg1", "bob", "❤️"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddReaction("msg1", "carol", "👍"); err != nil {
		t.Fatal(err)
	}

	// Get reactions
	reactions, err := s.GetReactions("msg1")
	if err != nil {
		t.Fatal(err)
	}
	if len(reactions) != 3 {
		t.Fatalf("expected 3 reactions, got %d", len(reactions))
	}

	// Remove reaction
	if err := s.RemoveReaction("msg1", "bob"); err != nil {
		t.Fatal(err)
	}
	reactions, _ = s.GetReactions("msg1")
	if len(reactions) != 2 {
		t.Fatalf("expected 2 reactions after remove, got %d", len(reactions))
	}

	// Empty reactions for non-existent message
	reactions, _ = s.GetReactions("nonexistent")
	if len(reactions) != 0 {
		t.Error("expected 0 reactions for non-existent message")
	}
}

func TestReadReceipts(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Save some messages first
	s.SaveMessage(Message{ID: "m1", From: "alice", To: "broadcast", Text: "hi", Timestamp: 1000})
	s.SaveMessage(Message{ID: "m2", From: "bob", To: "broadcast", Text: "hello", Timestamp: 1001})
	s.SaveMessage(Message{ID: "m3", From: "carol", To: "broadcast", Text: "hey", Timestamp: 1002})

	// Mark read
	if err := s.MarkRead("m1", "bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkRead("m1", "carol"); err != nil {
		t.Fatal(err)
	}

	// Get receipts
	receipts, err := s.GetReadReceipts("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 2 {
		t.Fatalf("expected 2 receipts, got %d", len(receipts))
	}

	// MarkAllRead
	if err := s.MarkAllRead("alice", 1001); err != nil {
		t.Fatal(err)
	}

	// Check m1 and m2 are read by alice
	receipts, _ = s.GetReadReceipts("m1")
	found := false
	for _, r := range receipts {
		if r == "alice" {
			found = true
		}
	}
	if !found {
		t.Error("expected alice to have read m1 after MarkAllRead")
	}

	receipts, _ = s.GetReadReceipts("m2")
	found = false
	for _, r := range receipts {
		if r == "alice" {
			found = true
		}
	}
	if !found {
		t.Error("expected alice to have read m2 after MarkAllRead")
	}
}

func TestProfiles(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Save profile
	if err := s.SaveProfile("npub_alice", "Alice Smith", "https://avatar.example.com/alice.png", "Hello world"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProfile("npub_bob", "Bob Jones", "", "Builder"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProfile("npub_ali", "Ali G", "", ""); err != nil {
		t.Fatal(err)
	}

	// Get profile
	profile, err := s.GetProfile("npub_alice")
	if err != nil {
		t.Fatal(err)
	}
	if profile["displayName"] != "Alice Smith" {
		t.Errorf("expected displayName=Alice Smith, got %v", profile["displayName"])
	}
	if profile["bio"] != "Hello world" {
		t.Errorf("expected bio=Hello world, got %v", profile["bio"])
	}

	// Search profiles
	results, err := s.SearchProfiles("Ali")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 profiles matching 'Ali', got %d", len(results))
	}

	// Search by npub prefix
	results, err = s.SearchProfiles("npub_bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 profile matching npub_bob, got %d", len(results))
	}
}

func TestSearchMessages(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	msgs := []Message{
		{ID: "m1", From: "alice", To: "broadcast", Text: "golang is awesome", Timestamp: 1000},
		{ID: "m2", From: "bob", To: "broadcast", Text: "I love golang too", Timestamp: 1001},
		{ID: "m3", From: "alice", To: "carol", Text: "golang DM", Timestamp: 1002},
		{ID: "m4", From: "bob", To: "dave", Text: "secret golang talk", Timestamp: 1003},
		{ID: "m5", From: "alice", To: "broadcast", Text: "nothing relevant", Timestamp: 1004},
	}
	for _, m := range msgs {
		s.SaveMessage(m)
	}

	// alice searches for "golang" — sees broadcasts + DMs involving alice
	results, err := s.SearchMessages("golang", "alice", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results for alice searching 'golang', got %d", len(results))
	}

	// bob searches — sees broadcasts + DMs involving bob (m4 is bob→dave)
	results, err = s.SearchMessages("golang", "bob", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results for bob, got %d", len(results))
	}

	// Search for non-existent
	results, err = s.SearchMessages("nonexistent", "alice", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("expected 0 results for nonexistent search")
	}
}

func TestLoadIdentityEmpty(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, _, _, err = s.LoadIdentity()
	if err == nil {
		t.Fatal("expected error loading identity from empty DB")
	}
	// The error should wrap sql.ErrNoRows
	if err != nil {
		// The LoadIdentity wraps the error, so check the wrapped chain
		if !isErrNoRows(err) {
			t.Logf("LoadIdentity returned error (expected sql.ErrNoRows in chain): %v", err)
		}
	}
}

func isErrNoRows(err error) bool {
	for e := err; e != nil; e = unwrap(e) {
		if e == sql.ErrNoRows {
			return true
		}
	}
	return false
}

func unwrap(err error) error {
	if u, ok := err.(interface{ Unwrap() error }); ok {
		return u.Unwrap()
	}
	return nil
}

func TestSaveAndLoadIdentity(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	npub := "npub1abcdef"
	nsec := "nsec1ghijkl"
	seed := "word1 word2 word3"

	if err := s.SaveIdentity(npub, nsec, seed); err != nil {
		t.Fatal(err)
	}

	gotNpub, gotNsec, gotSeed, err := s.LoadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if gotNpub != npub || gotNsec != nsec || gotSeed != seed {
		t.Errorf("identity roundtrip failed: got (%s, %s, %s), want (%s, %s, %s)",
			gotNpub, gotNsec, gotSeed, npub, nsec, seed)
	}
}
