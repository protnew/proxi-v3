package chat

import (
	"fmt"

	"nhooyr.io/websocket"
)

func isCallSignal(typ string) bool {
	switch typ {
	case TypeKeyExchange, "call-offer", "call-answer", "call-ice", "call-hangup", "call-end", "call-reject":
		return true
	default:
		return false
	}
}

func (h *ChatHub) deliverCall(msg *Message) error {
	h.mu.RLock()
	down := h.down
	h.mu.RUnlock()
	if down {
		return fmt.Errorf("hub down")
	}
	if h == nil || msg == nil {
		return fmt.Errorf("hub down")
	}
	if msg.To == "" || msg.To == BroadcastTarget {
		return fmt.Errorf("call requires peer")
	}
	if !h.IsOnline(msg.To) {
		return fmt.Errorf("peer offline")
	}
	encoded, err := msg.Encode()
	if err != nil {
		return err
	}
	if !h.SendTo(msg.To, encoded) {
		return fmt.Errorf("hub down")
	}
	if msg.Type == TypeKeyExchange || msg.Type == "call-offer" {
		ring := &Message{Type: "ringing", From: msg.To, To: msg.From, ID: msg.ID}
		out, err := ring.Encode()
		if err != nil {
			return err
		}
		if !h.SendTo(msg.From, out) {
			return fmt.Errorf("hub down")
		}
	}
	return nil
}

func (h *ChatHub) MarkDown() {
	h.mu.Lock()
	h.down = true
	var all []*Client
	for _, set := range h.clients {
		for c := range set {
			all = append(all, c)
		}
	}
	h.mu.Unlock()
	for _, c := range all {
		c.forceClose(websocket.StatusGoingAway, "hub down")
	}
}
