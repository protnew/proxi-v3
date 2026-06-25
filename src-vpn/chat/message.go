package chat

import (
	"bytes"
	"encoding/json"
	"time"
)

// Message types (JSON wire format string values)
const (
	TypeChat   = "chat"
	TypeJoin   = "join"
	TypeLeave  = "leave"
	TypeTyping = "typing"
	TypeVoice  = "voice" // voice message (JSON text fallback)
)

// Binary frame type byte — first byte of a binary WebSocket frame.
const BinaryFrameVoice byte = 0x02

// BinaryFrameStream is the prefix byte for binary stream video frames.
const BinaryFrameStream byte = 0x03

// BroadcastTarget is used in the "to" field to indicate a broadcast message.
const BroadcastTarget = "broadcast"

// MaxBinaryVoiceSize is the maximum allowed size for a binary voice frame (5 MB).
const MaxBinaryVoiceSize = 5 * 1024 * 1024

// Message represents a chat message exchanged over WebSocket.
//
// JSON wire format:
//
//	{"type":"chat|join|leave|typing|voice","from":"userId","to":"userId|broadcast","text":"...","ts":unix}
type Message struct {
	Type          string `json:"type"`                    // "chat", "join", "leave", "typing", "key_exchange", "message_edited", "message_deleted", "voice"
	From          string `json:"from"`                    // sender userId
	To            string `json:"to"`                      // receiver userId or "broadcast"
	Text          string `json:"text,omitempty"`          // message body (empty for join/leave/typing)
	Ts            int64  `json:"ts"`                      // unix timestamp
	PublicKey     string `json:"publicKey,omitempty"`     // ECDH public key for key_exchange
	ID            string `json:"id,omitempty"`            // message ID (for edit/delete/reply)
	ReplyTo       string `json:"replyTo,omitempty"`       // ID of message being replied to
	ReplyToText   string `json:"replyToText,omitempty"`   // preview text of replied message
	ReplyToFrom   string `json:"replyToFrom,omitempty"`   // sender of replied message
	ForwardedFrom string `json:"forwardedFrom,omitempty"` // original sender npub for forwarded messages
	TTL           int    `json:"ttl,omitempty"`           // self-destruct in seconds (0 = never)
	VoiceData     string `json:"voiceData,omitempty"`     // base64-encoded audio (legacy fallback)
	VoiceDuration int    `json:"voiceDuration,omitempty"` // voice duration in seconds
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

// VoiceMetadata is the JSON metadata embedded in a binary voice frame header.
type VoiceMetadata struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Duration int    `json:"duration"`
	Ts       int64  `json:"ts,omitempty"`
}

// EncodeBinaryVoice builds a binary WebSocket frame for a voice message.
// Wire format: [0x02] [JSON metadata] [0x00] [opus/webm audio bytes]
func EncodeBinaryVoice(meta *VoiceMetadata, audioData []byte) ([]byte, error) {
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 0, 1+len(metaBytes)+1+len(audioData))
	buf = append(buf, BinaryFrameVoice)
	buf = append(buf, metaBytes...)
	buf = append(buf, 0x00) // null terminator separates metadata from audio
	buf = append(buf, audioData...)
	return buf, nil
}

// DecodeBinaryVoice parses a binary voice frame.
// Returns metadata and raw audio bytes.
func DecodeBinaryVoice(data []byte) (*VoiceMetadata, []byte, error) {
	if len(data) < 2 || data[0] != BinaryFrameVoice {
		return nil, nil, ErrNotBinaryVoice
	}

	// Find null separator between metadata and audio
	rest := data[1:]
	nullIdx := bytes.IndexByte(rest, 0x00)
	if nullIdx < 0 {
		return nil, nil, ErrInvalidBinaryVoice
	}

	var meta VoiceMetadata
	if err := json.Unmarshal(rest[:nullIdx], &meta); err != nil {
		return nil, nil, ErrInvalidBinaryVoice
	}

	audioData := rest[nullIdx+1:]
	return &meta, audioData, nil
}

// IsBinaryVoiceFrame checks if a raw binary frame is a voice message.
func IsBinaryVoiceFrame(data []byte) bool {
	return len(data) > 0 && data[0] == BinaryFrameVoice
}

// BinaryVoiceToMessage converts a decoded binary voice frame into a
// standard Message (used for OnMessage callback and routing).
func BinaryVoiceToMessage(meta *VoiceMetadata, senderID string) *Message {
	to := meta.To
	if to == "" {
		to = BroadcastTarget
	}
	ts := meta.Ts
	if ts == 0 {
		ts = time.Now().Unix()
	}
	return &Message{
		Type:          TypeVoice,
		From:          senderID,
		To:            to,
		Text:          "🎤 Голосовое сообщение (" + itoa(meta.Duration) + "с)",
		Ts:            ts,
		VoiceDuration: meta.Duration,
	}
}

// itoa converts int to string without importing fmt (avoids extra dependency).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
