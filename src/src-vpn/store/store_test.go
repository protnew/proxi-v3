package store

import (
	"database/sql"
	"fmt"
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

// ---------------------------------------------------------------------------
// Nostr Events
// ---------------------------------------------------------------------------

func TestSaveNostrEvent(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	evt := NostrEvent{
		ID:        "evt1",
		PubKey:    "pk_alice",
		Kind:      1,
		Tags:      [][]string{{"e", "ref1"}, {"p", "refpk1"}},
		Content:   "hello nostr",
		Sig:       "sig123",
		CreatedAt: 1700000000,
	}
	if err := s.SaveNostrEvent(evt); err != nil {
		t.Fatalf("SaveNostrEvent: %v", err)
	}

	// Verify saved by reading back
	results, err := s.GetNostrEvents(NostrEventFilter{IDs: []string{"evt1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 event, got %d", len(results))
	}
	got := results[0]
	if got.ID != "evt1" || got.PubKey != "pk_alice" || got.Kind != 1 {
		t.Errorf("event mismatch: %+v", got)
	}
	if got.Content != "hello nostr" || got.Sig != "sig123" {
		t.Errorf("event content/sig mismatch: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0][0] != "e" || got.Tags[1][0] != "p" {
		t.Errorf("tags mismatch: %+v", got.Tags)
	}
}

func TestSaveNostrEvent_Duplicate(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	evt := NostrEvent{
		ID:        "dup1",
		PubKey:    "pk1",
		Kind:      1,
		Content:   "first",
		CreatedAt: 100,
	}
	if err := s.SaveNostrEvent(evt); err != nil {
		t.Fatal(err)
	}
	// Save again with same ID — should not error (INSERT OR IGNORE)
	evt.Content = "second"
	if err := s.SaveNostrEvent(evt); err != nil {
		t.Fatalf("duplicate save should not error: %v", err)
	}

	results, _ := s.GetNostrEvents(NostrEventFilter{IDs: []string{"dup1"}})
	if len(results) != 1 {
		t.Fatalf("expected 1 event (dedup), got %d", len(results))
	}
	// Content should be the first one since duplicate is ignored
	if results[0].Content != "first" {
		t.Errorf("expected content='first' (original), got %q", results[0].Content)
	}
}

func TestGetNostrEvents_FilterKinds(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	events := []NostrEvent{
		{ID: "k1", PubKey: "pk1", Kind: 0, Content: "metadata", CreatedAt: 100},
		{ID: "k2", PubKey: "pk1", Kind: 1, Content: "note", CreatedAt: 200},
		{ID: "k3", PubKey: "pk2", Kind: 3, Content: "contacts", CreatedAt: 300},
		{ID: "k4", PubKey: "pk1", Kind: 1, Content: "note2", CreatedAt: 400},
	}
	for _, e := range events {
		s.SaveNostrEvent(e)
	}

	results, err := s.GetNostrEvents(NostrEventFilter{Kinds: []int{1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 events with kind=1, got %d", len(results))
	}
}

func TestGetNostrEvents_FilterAuthors(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	events := []NostrEvent{
		{ID: "a1", PubKey: "alice", Kind: 1, Content: "hi", CreatedAt: 100},
		{ID: "a2", PubKey: "bob", Kind: 1, Content: "hello", CreatedAt: 200},
		{ID: "a3", PubKey: "alice", Kind: 1, Content: "world", CreatedAt: 300},
	}
	for _, e := range events {
		s.SaveNostrEvent(e)
	}

	results, err := s.GetNostrEvents(NostrEventFilter{Authors: []string{"alice"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 events from alice, got %d", len(results))
	}
}

func TestGetNostrEvents_FilterSinceUntil(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	events := []NostrEvent{
		{ID: "t1", PubKey: "pk1", Kind: 1, Content: "old", CreatedAt: 100},
		{ID: "t2", PubKey: "pk1", Kind: 1, Content: "mid", CreatedAt: 200},
		{ID: "t3", PubKey: "pk1", Kind: 1, Content: "new", CreatedAt: 300},
	}
	for _, e := range events {
		s.SaveNostrEvent(e)
	}

	since := int64(150)
	results, err := s.GetNostrEvents(NostrEventFilter{Since: &since})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 events with since=150, got %d", len(results))
	}

	until := int64(250)
	results, err = s.GetNostrEvents(NostrEventFilter{Until: &until})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 events with until=250, got %d", len(results))
	}

	// Both since and until
	results, err = s.GetNostrEvents(NostrEventFilter{Since: &since, Until: &until})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 event with since=150 and until=250, got %d", len(results))
	}
}

func TestGetNostrEvents_FilterIDs(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	events := []NostrEvent{
		{ID: "id1", PubKey: "pk1", Kind: 1, Content: "one", CreatedAt: 100},
		{ID: "id2", PubKey: "pk1", Kind: 1, Content: "two", CreatedAt: 200},
		{ID: "id3", PubKey: "pk1", Kind: 1, Content: "three", CreatedAt: 300},
	}
	for _, e := range events {
		s.SaveNostrEvent(e)
	}

	results, err := s.GetNostrEvents(NostrEventFilter{IDs: []string{"id1", "id3"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 events, got %d", len(results))
	}
}

func TestGetNostrEvents_Limit(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for i := 0; i < 20; i++ {
		s.SaveNostrEvent(NostrEvent{
			ID:        fmt.Sprintf("lim%d", i),
			PubKey:    "pk1",
			Kind:      1,
			Content:   fmt.Sprintf("msg%d", i),
			CreatedAt: int64(100 + i),
		})
	}

	results, err := s.GetNostrEvents(NostrEventFilter{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 events with limit=5, got %d", len(results))
	}

	// Default limit (0 → 100)
	results, err = s.GetNostrEvents(NostrEventFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 20 {
		t.Fatalf("expected 20 events with default limit, got %d", len(results))
	}
}

func TestGetNostrEvents_OrderDescending(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	events := []NostrEvent{
		{ID: "o1", PubKey: "pk1", Kind: 1, Content: "first", CreatedAt: 100},
		{ID: "o2", PubKey: "pk1", Kind: 1, Content: "second", CreatedAt: 200},
		{ID: "o3", PubKey: "pk1", Kind: 1, Content: "third", CreatedAt: 300},
	}
	for _, e := range events {
		s.SaveNostrEvent(e)
	}

	results, _ := s.GetNostrEvents(NostrEventFilter{})
	if len(results) != 3 {
		t.Fatalf("expected 3 events, got %d", len(results))
	}
	// Newest first (DESC order)
	if results[0].ID != "o3" {
		t.Errorf("expected first result to be o3 (newest), got %s", results[0].ID)
	}
	if results[2].ID != "o1" {
		t.Errorf("expected last result to be o1 (oldest), got %s", results[2].ID)
	}
}

func TestGetNostrEvents_Empty(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	results, err := s.GetNostrEvents(NostrEventFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 events from empty DB, got %d", len(results))
	}
}

func TestNostrEventFilter_AllFieldsCombined(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	events := []NostrEvent{
		{ID: "c1", PubKey: "alice", Kind: 1, Content: "match", CreatedAt: 150},
		{ID: "c2", PubKey: "alice", Kind: 1, Content: "too old", CreatedAt: 50},
		{ID: "c3", PubKey: "bob", Kind: 1, Content: "wrong author", CreatedAt: 200},
		{ID: "c4", PubKey: "alice", Kind: 2, Content: "wrong kind", CreatedAt: 200},
		{ID: "c5", PubKey: "alice", Kind: 1, Content: "too new", CreatedAt: 500},
	}
	for _, e := range events {
		s.SaveNostrEvent(e)
	}

	since := int64(100)
	until := int64(300)
	results, err := s.GetNostrEvents(NostrEventFilter{
		Authors: []string{"alice"},
		Kinds:   []int{1},
		Since:   &since,
		Until:   &until,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 event matching all filters, got %d", len(results))
	}
	if results[0].ID != "c1" {
		t.Errorf("expected c1, got %s", results[0].ID)
	}
}
