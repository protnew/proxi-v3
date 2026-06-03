package analytics

import (
	"path/filepath"
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

func newTestTracker(t *testing.T) *Tracker {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return NewTracker(s)
}

func TestTrackEvent(t *testing.T) {
	tr := newTestTracker(t)

	err := tr.TrackEvent("user1", "login", map[string]interface{}{"ip": "127.0.0.1"})
	if err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}

	err = tr.TrackEvent("user1", "message_sent", map[string]interface{}{"channel": "general"})
	if err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}

	err = tr.TrackEvent("user2", "login", nil)
	if err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}
}

func TestGetDAU(t *testing.T) {
	tr := newTestTracker(t)

	// Track events for two users
	_ = tr.TrackEvent("user1", "login", nil)
	_ = tr.TrackEvent("user2", "login", nil)

	dau, err := tr.GetDAU(1)
	if err != nil {
		t.Fatalf("GetDAU: %v", err)
	}
	if dau != 2 {
		t.Errorf("DAU = %d, want 2", dau)
	}
}

func TestGetMAU(t *testing.T) {
	tr := newTestTracker(t)

	_ = tr.TrackEvent("user1", "login", nil)
	_ = tr.TrackEvent("user2", "login", nil)
	_ = tr.TrackEvent("user3", "login", nil)

	mau, err := tr.GetMAU()
	if err != nil {
		t.Fatalf("GetMAU: %v", err)
	}
	if mau != 3 {
		t.Errorf("MAU = %d, want 3", mau)
	}
}

func TestGetMessagesPerDay(t *testing.T) {
	tr := newTestTracker(t)

	_ = tr.TrackEvent("user1", "message_sent", nil)
	_ = tr.TrackEvent("user1", "message_sent", nil)
	_ = tr.TrackEvent("user2", "message_sent", nil)

	msgs, err := tr.GetMessagesPerDay(1)
	if err != nil {
		t.Fatalf("GetMessagesPerDay: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("expected at least one day of messages")
	}
	// Should have 3 messages for today
	for _, m := range msgs {
		if cnt, ok := m["count"].(int); ok && cnt != 3 {
			t.Errorf("message count = %d, want 3", cnt)
		}
	}
}

func TestGetRetention(t *testing.T) {
	tr := newTestTracker(t)

	// Track events with explicit cohort
	_ = tr.TrackEvent("user1", "signup", nil)

	// Retention for far-future cohort with no users = 0
	ret, err := tr.GetRetention("2099-01-01")
	if err != nil {
		t.Fatalf("GetRetention: %v", err)
	}
	if ret != 0 {
		t.Errorf("retention for empty cohort = %f, want 0", ret)
	}
}
