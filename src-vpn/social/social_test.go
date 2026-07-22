package social

import (
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

func newTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateAndGetStories(t *testing.T) {
	db := newTestDB(t)
	CreateStory(db, "alice", "https://img.example.com/1.jpg", "image")
	CreateStory(db, "bob", "https://img.example.com/2.mp4", "video")

	stories, err := GetActiveStories(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(stories) != 2 {
		t.Fatalf("expected 2 stories, got %d", len(stories))
	}
}

func TestDeleteExpiredStories(t *testing.T) {
	db := newTestDB(t)
	// Insert expired story directly
	db.DB().Exec("INSERT INTO stories (author, media_url, media_type, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		"alice", "url", "image", time.Now().Unix()-100, time.Now().Unix()-10)
	// Insert active story
	CreateStory(db, "bob", "url2", "image")

	deleted, _ := DeleteExpiredStories(db)
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	active, _ := GetActiveStories(db)
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
}

func TestThreads(t *testing.T) {
	db := newTestDB(t)
	AddReply(db, "msg1", "msg2")
	AddReply(db, "msg1", "msg3")

	replies, err := GetThread(db, "msg1")
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(replies))
	}
	if replies[0].ReplyID != "msg2" {
		t.Errorf("expected msg2, got %s", replies[0].ReplyID)
	}
}
