package chat

import "encoding/json"

// Message types
const (
	TypeChat   = "chat"
	TypeJoin   = "join"
	TypeLeave  = "leave"
	TypeTyping = "typing"
)

// BroadcastTarget is used in the "to" field to indicate a broadcast message.
const BroadcastTarget = "broadcast"

// Message represents a chat message exchanged over WebSocket.
//
// JSON wire format:
//
//	{"type":"chat|join|leave|typing","from":"userId","to":"userId|broadcast","text":"...","ts":unix}
type Message struct {
	Type      string `json:"type"`                 // "chat", "join", "leave", "typing", "key_exchange"
	From      string `json:"from"`                 // sender userId
	To        string `json:"to"`                   // receiver userId or "broadcast"
	Text      string `json:"text,omitempty"`       // message body (empty for join/leave/typing)
	Ts        int64  `json:"ts"`                   // unix timestamp
	PublicKey string `json:"publicKey,omitempty"`  // ECDH public key for key_exchange
}

// Encode marshals the message to JSON bytes.
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage unmarshals JSON bytes into a Message.
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
