package chat

import (
	"encoding/json"
	"testing"
	"time"
)

// TestAliceBobDMDelivery verifies hub routes DM Alice→Bob (MSG-008).
func TestAliceBobDMDelivery(t *testing.T) {
	hub := NewChatHub()

	aliceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	bobID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	alice := &Client{UserID: aliceID, Send: make(chan []byte, 32), hub: hub}
	bob := &Client{UserID: bobID, Send: make(chan []byte, 32), hub: hub}
	hub.Register(alice)
	hub.Register(bob)
	defer hub.Unregister(alice)
	defer hub.Unregister(bob)

	// Drain welcome/join noise
	drain := func(c *Client) {
		for {
			select {
			case <-c.Send:
			default:
				return
			}
		}
	}
	drain(alice)
	drain(bob)

	if !hub.IsOnline(aliceID) || !hub.IsOnline(bobID) {
		t.Fatal("both must be online")
	}

	payload, err := json.Marshal(&Message{
		Type: "chat",
		From: aliceID,
		To:   bobID,
		Text: "hello bob from alice",
		Ts:   time.Now().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !hub.SendTo(bobID, payload) {
		t.Fatal("SendTo bob failed — routing key mismatch?")
	}
	hub.SendTo(aliceID, payload) // echo to sender

	assertChat := func(name string, ch <-chan []byte) {
		t.Helper()
		select {
		case got := <-ch:
			var m Message
			if err := json.Unmarshal(got, &m); err != nil {
				t.Fatalf("%s decode: %v", name, err)
			}
			if m.Text != "hello bob from alice" {
				t.Fatalf("%s text=%q", name, m.Text)
			}
			if m.From != aliceID {
				t.Fatalf("%s from=%s", name, m.From)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s did not receive DM", name)
		}
	}
	assertChat("bob", bob.Send)
	assertChat("alice", alice.Send)
}

func TestSendToUnknownUser(t *testing.T) {
	hub := NewChatHub()
	ok := hub.SendTo("missing-user", []byte(`{"type":"chat"}`))
	if ok {
		t.Fatal("expected false for offline user")
	}
}
