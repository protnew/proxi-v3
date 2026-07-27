package store

import (
	"testing"
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

func TestDeleteEmptyMessages(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// bypass SaveMessage guard via raw insert
	_, err = s.DB().Exec(`INSERT INTO messages (id, sender, recipient, text, encrypted, timestamp) VALUES
		('x1','a','b','',0,1),
		('x2','a','b','hi',0,2),
		('x3','a','b','  ',0,3)`)
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.DeleteEmptyMessages()
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("deleted=%d want >=1", n)
	}
	msgs, err := s.GetMessages(50, 0, "a")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range msgs {
		if m.Text == "" || m.Text == "  " {
			t.Fatalf("empty still present: %+v", m)
		}
	}
}
