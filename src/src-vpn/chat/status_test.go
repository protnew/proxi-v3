package chat

import (
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestSetGetStatus — verifies Queued → Sent → Delivered → Read transitions
// ---------------------------------------------------------------------------

func TestSetGetStatus(t *testing.T) {
	t.Parallel()

	msgID := "msg-status-001"

	// Before any Set, status should be Queued.
	if s := GetMessageStatus(msgID); s != Queued {
		t.Fatalf("expected Queued for unknown msg, got %d", s)
	}

	// Queued
	SetMessageStatus(msgID, Queued)
	if s := GetMessageStatus(msgID); s != Queued {
		t.Fatalf("expected Queued, got %d", s)
	}

	// Sent
	SetMessageStatus(msgID, Sent)
	if s := GetMessageStatus(msgID); s != Sent {
		t.Fatalf("expected Sent, got %d", s)
	}

	// Delivered
	SetMessageStatus(msgID, Delivered)
	if s := GetMessageStatus(msgID); s != Delivered {
		t.Fatalf("expected Delivered, got %d", s)
	}

	// Read
	SetMessageStatus(msgID, Read)
	if s := GetMessageStatus(msgID); s != Read {
		t.Fatalf("expected Read, got %d", s)
	}

	// Different msgID should still be Queued
	if s := GetMessageStatus("other-msg"); s != Queued {
		t.Fatalf("expected Queued for other msg, got %d", s)
	}

	// Clean up so other parallel tests don't see our entries
	statusStore.Delete(msgID)
}

// ---------------------------------------------------------------------------
// TestTypingBroadcast — 2 clients, one types, the other receives
// ---------------------------------------------------------------------------

func TestTypingBroadcast(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	// Create two clients
	_, sConn1, cleanup1 := newTestWSConn(t)
	defer cleanup1()
	c1 := &Client{UserID: "typer", Conn: sConn1, Send: make(chan []byte, SendChannelSize)}
	hub.Register(c1)
	drainAll(c1.Send, t)

	_, sConn2, cleanup2 := newTestWSConn(t)
	defer cleanup2()
	c2 := &Client{UserID: "reader", Conn: sConn2, Send: make(chan []byte, SendChannelSize)}
	hub.Register(c2)
	// Drain: c2 gets welcome+join for self, c1 gets join broadcast for c2
	drainAll(c2.Send, t)
	drainAll(c1.Send, t)

	// c1 types in channel "ch1"
	BroadcastTyping(hub, "ch1", "typer")

	// c2 should receive the typing event
	select {
	case data := <-c2.Send:
		var env struct {
			Type      string `json:"type"`
			ChannelID string `json:"channelID"`
			UserID    string `json:"userID"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			t.Fatalf("unmarshal typing event: %v", err)
		}
		if env.Type != "typing" {
			t.Fatalf("expected type=typing, got %s", env.Type)
		}
		if env.ChannelID != "ch1" {
			t.Fatalf("expected channelID=ch1, got %s", env.ChannelID)
		}
		if env.UserID != "typer" {
			t.Fatalf("expected userID=typer, got %s", env.UserID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reader did not receive typing event")
	}

	// c1 (sender) should NOT receive it (excluded)
	assertNoReceive(t, c1.Send, "typer")
}
