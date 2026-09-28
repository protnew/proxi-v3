package chat

import (
	"encoding/json"
	"sync"
	"time"
)

// MessageStatus represents the delivery status of a chat message.
type MessageStatus int

const (
	// Queued means the message is waiting to be sent.
	Queued MessageStatus = iota
	// Sent means the message was sent to the server.
	Sent
	// Delivered means the message was delivered to the recipient's device.
	Delivered
	// Read means the recipient has read the message.
	Read
)

// statusStore holds per-message statuses in a concurrent-safe map.
var statusStore sync.Map

// SetMessageStatus stores the status for a given message ID.
func SetMessageStatus(msgID string, status MessageStatus) {
	statusStore.Store(msgID, status)
}

// GetMessageStatus retrieves the status for a given message ID.
// Returns Queued if no status has been recorded.
func GetMessageStatus(msgID string) MessageStatus {
	v, ok := statusStore.Load(msgID)
	if !ok {
		return Queued
	}
	s, ok := v.(MessageStatus)
	if !ok {
		return Queued
	}
	return s
}

// TypingEvent represents a user-typing notification in a channel.
type TypingEvent struct {
	ChannelID string `json:"channelID"`
	UserID    string `json:"userID"`
	Timestamp int64  `json:"timestamp"`
}

// typingEnvelope is the JSON wrapper broadcast to channel participants.
type typingEnvelope struct {
	Type      string `json:"type"`
	ChannelID string `json:"channelID"`
	UserID    string `json:"userID"`
}

// BroadcastTyping sends a typing indicator to all clients in the hub
// (except the typing user). The payload is {"type":"typing","channelID":"...","userID":"..."}.
func BroadcastTyping(hub *ChatHub, channelID, userID string) {
	env := typingEnvelope{
		Type:      "typing",
		ChannelID: channelID,
		UserID:    userID,
	}
	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	_ = TypingEvent{ChannelID: channelID, UserID: userID, Timestamp: time.Now().Unix()}
	hub.Broadcast(data, userID)
}
