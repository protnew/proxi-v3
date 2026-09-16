package store

import (
	"testing"
	"time"
)

func TestSaveMessageSkipsEmpty(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.SaveMessage(Message{ID: "e1", From: "a", To: "b", Text: "", Timestamp: 1}); err != nil {
		t.Fatalf("empty save: %v", err)
	}
	if err := s.SaveMessage(Message{ID: "e2", From: "a", To: "b", Text: "   ", Timestamp: 2}); err != nil {
		t.Fatalf("ws save: %v", err)
	}
	if err := s.SaveMessage(Message{ID: "ok1", From: "a", To: "b", Text: "hello", Timestamp: 3}); err != nil {
		t.Fatalf("ok save: %v", err)
	}

	msgs, err := s.GetMessages(50, 0, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Text != "hello" {
		t.Fatalf("got %+v want single hello", msgs)
	}
}

func TestDeleteEmptyMessagesRequiresErasureMarker(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Now().Unix()
	_, err = s.DB().Exec(`INSERT INTO messages
		(id, sender, recipient, text, encrypted, timestamp, ttl, erased_at) VALUES
		('empty','a','b','',0,1,1,0),
		('space','a','b','  ',0,2,1,0),
		('marked-nottl','a','b','',0,3,0,123),
		('marked-fresh','a','b','',0,?,999999,123),
		('marked-exp','a','b','',0,1,1,123)`, now)
	if err != nil {
		t.Fatal(err)
	}

	n, err := s.DeleteEmptyMessages(false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("allowEmptyCleanup=false deleted=%d want 0", n)
	}

	n, err = s.DeleteEmptyMessages(true)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted=%d want 1 expired+erased_at row", n)
	}

	var survivors int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM messages WHERE id IN ('empty','space','marked-nottl','marked-fresh')`).Scan(&survivors); err != nil {
		t.Fatal(err)
	}
	if survivors != 4 {
		t.Fatalf("unmarked/not-expired survivors=%d want 4", survivors)
	}
	var gone int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM messages WHERE id = 'marked-exp'`).Scan(&gone); err != nil {
		t.Fatal(err)
	}
	if gone != 0 {
		t.Fatalf("marked-exp still present")
	}
}

func TestDeleteEmptyDoesNotTouchOrdinaryEmpty(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, err = s.DB().Exec(`INSERT INTO messages
		(id, sender, recipient, text, encrypted, timestamp, ttl, erased_at) VALUES
		('plain-empty','a','b','',0,1,1,0)`)
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.DeleteEmptyMessages(true)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("ordinary empty without erased_at deleted=%d want 0", n)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM messages WHERE id = 'plain-empty'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("ordinary empty row was removed without erased_at")
	}
}

func TestSoftDeleteHidesMessage(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.SaveMessage(Message{ID: "keep", From: "alice", To: "bob", Text: "visible", Timestamp: 10}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMessage(Message{ID: "gone", From: "alice", To: "bob", Text: "hide me", Timestamp: 11}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteMessage("gone", "alice"); err != nil {
		t.Fatal(err)
	}

	msgs, err := s.GetMessages(50, 0, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].ID != "keep" {
		t.Fatalf("GetMessages after soft-delete: %+v", msgs)
	}
	if _, err := s.GetMessageByID("gone"); err == nil {
		t.Fatal("GetMessageByID should hide soft-deleted row")
	}

	var deletedAt int64
	if err := s.DB().QueryRow(`SELECT deleted_at FROM messages WHERE id = 'gone'`).Scan(&deletedAt); err != nil {
		t.Fatalf("tombstone missing: %v", err)
	}
	if deletedAt <= 0 {
		t.Fatalf("deleted_at=%d want > 0", deletedAt)
	}
}

func TestCryptoEraseMessageRollsBackWipeWhenDeleteFails(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.SaveMessage(Message{ID: "secret", From: "a", To: "b", Text: "keep on rollback", Timestamp: 1}); err != nil {
		t.Fatal(err)
	}
	_, err = s.DB().Exec(`CREATE TRIGGER block_sec_delete BEFORE DELETE ON messages
		WHEN OLD.erased_at > 0 BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CryptoEraseMessage("secret"); err == nil {
		t.Fatal("expected injected delete failure")
	}

	var text string
	var erasedAt int64
	if err := s.DB().QueryRow(`SELECT text, erased_at FROM messages WHERE id = 'secret'`).Scan(&text, &erasedAt); err != nil {
		t.Fatal(err)
	}
	if text != "keep on rollback" || erasedAt != 0 {
		t.Fatalf("transaction did not roll back: text=%q erased_at=%d", text, erasedAt)
	}
	if n, err := s.DeleteEmptyMessages(true); err != nil || n != 0 {
		t.Fatalf("cleanup after rolled-back erase: deleted=%d err=%v", n, err)
	}
}
