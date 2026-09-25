package main

import (
	"fmt"
	"strings"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/store"
)

func blindLogLine(msg *chat.Message) string {
	if msg == nil {
		return "chat nil"
	}
	return fmt.Sprintf("type=%s encrypted=%v len=%d", msg.Type, msg.ClaimedEncrypted(), len(msg.Text))
}

func saveIncomingChat(db *store.Store, msg *chat.Message) error {
	if db == nil || msg == nil || msg.Type != chat.TypeChat || strings.TrimSpace(msg.Text) == "" {
		return nil
	}
	if msg.ClaimedEncrypted() && !chat.LooksLikeClientCiphertext(msg.Text) {
		return fmt.Errorf("refuse plaintext under encrypted")
	}
	msg.NormalizeE2EFlags()
	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = fmt.Sprintf("msg-%d-%s", msg.Ts, randomHex(4))
	}
	return db.SaveMessage(store.Message{
		ID:            msg.ID,
		From:          msg.From,
		To:            msg.To,
		Text:          msg.Text,
		Encrypted:     msg.ClaimedEncrypted(),
		Timestamp:     msg.Ts,
		ReplyTo:       msg.ReplyTo,
		ForwardedFrom: msg.ForwardedFrom,
		TTL:           msg.TTL,
		Group:         msg.Group,
	})
}
