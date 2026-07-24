package msgops

import (
	"testing"
	"time"
)

func TestEditMessage(t *testing.T) {
	m := NewManager()
	edit, err := m.EditMessage("msg1", "user1", "Hello world")
	if err != nil {
		t.Fatal(err)
	}
	if edit.Text != "Hello world" {
		t.Fatalf("text: got %q", edit.Text)
	}
	if edit.EditedN != 1 {
		t.Fatalf("edit count: got %d", edit.EditedN)
	}

	// Second edit
	edit2, err := m.EditMessage("msg1", "user1", "Hello world!")
	if err != nil {
		t.Fatal(err)
	}
	if edit2.EditedN != 2 {
		t.Fatalf("edit count: got %d", edit2.EditedN)
	}
	if edit2.PrevText != "Hello world" {
		t.Fatalf("prev text: got %q", edit2.PrevText)
	}
}

func TestDeleteMessage(t *testing.T) {
	m := NewManager()
	err := m.DeleteMessage("msg1", "user1")
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsDeleted("msg1") {
		t.Fatal("should be deleted")
	}

	// Can't edit deleted
	_, err = m.EditMessage("msg1", "user1", "hack")
	if err != ErrMessageDeleted {
		t.Fatalf("expected ErrMessageDeleted, got %v", err)
	}

	// Double delete
	err = m.DeleteMessage("msg1", "user1")
	if err != ErrAlreadyDeleted {
		t.Fatalf("expected ErrAlreadyDeleted, got %v", err)
	}
}

func TestTyping(t *testing.T) {
	m := NewManager()
	m.SetTyping("user1", "room1")
	m.SetTyping("user2", "room1")

	typing := m.GetTyping("room1", "user1")
	if len(typing) != 1 {
		t.Fatalf("typing count: got %d", len(typing))
	}
	if typing[0].UserID != "user2" {
		t.Fatalf("typing user: got %q", typing[0].UserID)
	}
}

func TestTypingExpiry(t *testing.T) {
	m := NewManager()
	m.mu.Lock()
	m.typing["room1:user1"] = &TypingStatus{
		UserID:    "user1",
		RoomID:    "room1",
		Timestamp: time.Now().Unix() - 10, // 10 sec ago
	}
	m.mu.Unlock()

	typing := m.GetTyping("room1", "")
	if len(typing) != 0 {
		t.Fatalf("expired typing should be 0, got %d", len(typing))
	}
}

func TestScheduleMessage(t *testing.T) {
	m := NewManager()
	sendAt := time.Now().Unix() + 3600 // 1 hour from now

	s := m.ScheduleMessage("user1", "room1", "Happy birthday!", sendAt)
	if s.ID == "" {
		t.Fatal("empty ID")
	}
	if s.Sent {
		t.Fatal("should not be sent yet")
	}

	// Not yet time
	pending := m.GetPendingScheduled()
	if len(pending) != 0 {
		t.Fatalf("pending: got %d", len(pending))
	}

	// List scheduled
	list := m.ListScheduled("user1")
	if len(list) != 1 {
		t.Fatalf("scheduled list: got %d", len(list))
	}
}

func TestScheduleMessagePast(t *testing.T) {
	m := NewManager()
	sendAt := time.Now().Unix() - 10 // 10 sec ago

	m.ScheduleMessage("user1", "room1", "Late msg", sendAt)
	pending := m.GetPendingScheduled()
	if len(pending) != 1 {
		t.Fatalf("should be pending: got %d", len(pending))
	}
}

func TestCancelScheduled(t *testing.T) {
	m := NewManager()
	s := m.ScheduleMessage("user1", "room1", "Cancel me", time.Now().Unix()+3600)

	err := m.CancelScheduled(s.ID, "user1")
	if err != nil {
		t.Fatal(err)
	}

	list := m.ListScheduled("user1")
	if len(list) != 0 {
		t.Fatalf("should be empty after cancel: got %d", len(list))
	}

	// Cancel non-existent
	err = m.CancelScheduled("fake", "user1")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Cancel other user's message
	s2 := m.ScheduleMessage("user2", "room1", "Not yours", time.Now().Unix()+3600)
	err = m.CancelScheduled(s2.ID, "user1")
	if err != ErrNotOwner {
		t.Fatalf("expected ErrNotOwner, got %v", err)
	}
}

func TestEventSubscribe(t *testing.T) {
	m := NewManager()
	received := make([]string, 0)

	m.OnSubscribe(func(eventType string, payload []byte) {
		received = append(received, eventType)
	})

	m.DeleteMessage("msg1", "user1")
	m.SetTyping("user1", "room1")

	// Give goroutines time
	time.Sleep(50 * time.Millisecond)
	if len(received) < 2 {
		t.Fatalf("events: got %d", len(received))
	}
}
