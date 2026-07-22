package chat

import (
	"context"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// ---------------------------------------------------------------------------
// TestBinaryVoiceEncodeDecode tests the binary voice frame encoder/decoder.
// ---------------------------------------------------------------------------

func TestBinaryVoiceEncodeDecode(t *testing.T) {
	t.Parallel()

	meta := &VoiceMetadata{
		From:     "alice",
		To:       "broadcast",
		Duration: 15,
		Ts:       time.Now().Unix(),
	}
	audioData := []byte("fake-opus-audio-data-here")

	frame, err := EncodeBinaryVoice(meta, audioData)
	if err != nil {
		t.Fatalf("EncodeBinaryVoice: %v", err)
	}

	// Verify first byte is 0x02
	if frame[0] != BinaryFrameVoice {
		t.Fatalf("expected first byte 0x02, got 0x%02x", frame[0])
	}

	// Decode
	gotMeta, gotAudio, err := DecodeBinaryVoice(frame)
	if err != nil {
		t.Fatalf("DecodeBinaryVoice: %v", err)
	}

	if gotMeta.From != "alice" {
		t.Fatalf("expected from=alice, got %s", gotMeta.From)
	}
	if gotMeta.To != "broadcast" {
		t.Fatalf("expected to=broadcast, got %s", gotMeta.To)
	}
	if gotMeta.Duration != 15 {
		t.Fatalf("expected duration=15, got %d", gotMeta.Duration)
	}

	if string(gotAudio) != string(audioData) {
		t.Fatalf("audio data mismatch: got %q, want %q", string(gotAudio), string(audioData))
	}
}

// ---------------------------------------------------------------------------
// TestIsBinaryVoiceFrame
// ---------------------------------------------------------------------------

func TestIsBinaryVoiceFrame(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"voice frame", []byte{0x02, '{', '}', 0x00, 0xFF}, true},
		{"empty", []byte{}, false},
		{"wrong type", []byte{0x01, 0x00}, false},
		{"text JSON", []byte(`{"type":"chat"}`), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinaryVoiceFrame(tt.data)
			if got != tt.want {
				t.Errorf("IsBinaryVoiceFrame(%x) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestDecodeBinaryVoiceErrors tests error cases.
// ---------------------------------------------------------------------------

func TestDecodeBinaryVoiceErrors(t *testing.T) {
	t.Parallel()

	// Not a voice frame (wrong type byte)
	_, _, err := DecodeBinaryVoice([]byte{0x01, 0x00})
	if err != ErrNotBinaryVoice {
		t.Fatalf("expected ErrNotBinaryVoice, got %v", err)
	}

	// Too short (only 1 byte)
	_, _, err = DecodeBinaryVoice([]byte{0x02})
	if err != ErrNotBinaryVoice {
		t.Fatalf("expected ErrNotBinaryVoice for 1-byte frame, got %v", err)
	}

	// No null terminator — all bytes after 0x02 are non-null
	_, _, err = DecodeBinaryVoice([]byte{0x02, '{', '"', 'x', '"', '}', 0x01, 0x02})
	if err != ErrInvalidBinaryVoice {
		t.Fatalf("expected ErrInvalidBinaryVoice for no null, got %v", err)
	}

	// Invalid JSON after null separator
	_, _, err = DecodeBinaryVoice([]byte{0x02, 'b', 'a', 'd', 0x00, 0x01})
	if err != ErrInvalidBinaryVoice {
		t.Fatalf("expected ErrInvalidBinaryVoice for bad JSON, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestBinaryVoiceToMessage tests the conversion helper.
// ---------------------------------------------------------------------------

func TestBinaryVoiceToMessage(t *testing.T) {
	t.Parallel()

	meta := &VoiceMetadata{
		From:     "",
		To:       "",
		Duration: 30,
		Ts:       0,
	}

	msg := BinaryVoiceToMessage(meta, "bob")

	if msg.Type != TypeVoice {
		t.Fatalf("expected type=%s, got %s", TypeVoice, msg.Type)
	}
	if msg.From != "bob" {
		t.Fatalf("expected from=bob, got %s", msg.From)
	}
	if msg.To != BroadcastTarget {
		t.Fatalf("expected to=broadcast, got %s", msg.To)
	}
	if msg.VoiceDuration != 30 {
		t.Fatalf("expected duration=30, got %d", msg.VoiceDuration)
	}
	if msg.Ts == 0 {
		t.Fatal("expected non-zero timestamp")
	}
}

// ---------------------------------------------------------------------------
// TestBinaryVoiceRoundTripThroughHub verifies that a binary voice frame
// broadcast reaches the intended recipient.
// ---------------------------------------------------------------------------

func TestBinaryVoiceRoundTripThroughHub(t *testing.T) {
	t.Parallel()

	hub := NewChatHub()

	// Create two clients
	_, sConn1, cleanup1 := newTestWSConn(t)
	defer cleanup1()
	c1 := &Client{UserID: "sender", Conn: sConn1, Send: make(chan []byte, SendChannelSize)}
	hub.Register(c1)
	drainAll(c1.Send, t)

	_, sConn2, cleanup2 := newTestWSConn(t)
	defer cleanup2()
	c2 := &Client{UserID: "receiver", Conn: sConn2, Send: make(chan []byte, SendChannelSize)}
	hub.Register(c2)
	drainAll(c2.Send, t)

	// The registration of c2 sends a join broadcast to c1 — drain that.
	drainAll(c1.Send, t)

	// Build a binary voice frame
	meta := &VoiceMetadata{
		From:     "sender",
		To:       "broadcast",
		Duration: 5,
		Ts:       time.Now().Unix(),
	}
	audioData := []byte("opus-bytes")
	frame, err := EncodeBinaryVoice(meta, audioData)
	if err != nil {
		t.Fatal(err)
	}

	// Broadcast excluding sender
	hub.Broadcast(frame, "sender")

	// c2 should receive the binary frame
	select {
	case data := <-c2.Send:
		if !IsBinaryVoiceFrame(data) {
			t.Fatal("expected binary voice frame on receiver")
		}
		gotMeta, gotAudio, err := DecodeBinaryVoice(data)
		if err != nil {
			t.Fatal(err)
		}
		if gotMeta.Duration != 5 {
			t.Fatalf("expected duration=5, got %d", gotMeta.Duration)
		}
		if string(gotAudio) != "opus-bytes" {
			t.Fatalf("audio mismatch: %q", string(gotAudio))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("receiver did not get binary voice frame")
	}

	// c1 (sender) should NOT receive it (excluded)
	assertNoReceive(t, c1.Send, "sender")
}

// ---------------------------------------------------------------------------
// TestBinaryVoiceWritePumpType verifies that WritePump sends binary frames
// with MessageBinary and text frames with MessageText.
// ---------------------------------------------------------------------------

func TestBinaryVoiceWritePumpType(t *testing.T) {
	hub := NewChatHub()

	clientConn, serverConn, cleanup := newTestWSConn(t)
	defer cleanup()

	c := &Client{
		UserID: "binTest",
		Conn:   serverConn,
		Send:   make(chan []byte, SendChannelSize),
	}
	hub.Register(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		c.Serve(ctx)
		close(done)
	}()

	// Read welcome + join announcement (both text JSON, sent by Register)
	for i := 0; i < 2; i++ {
		readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
		msgType, _, err := clientConn.Read(readCtx)
		readCancel()
		if err != nil {
			t.Fatalf("read msg %d: %v", i, err)
		}
		if msgType != websocket.MessageText {
			t.Fatalf("msg %d should be text, got %v", i, msgType)
		}
	}

	// Now send a binary voice frame through the hub to this client
	meta := &VoiceMetadata{From: "other", To: "binTest", Duration: 3, Ts: time.Now().Unix()}
	frame, _ := EncodeBinaryVoice(meta, []byte("audio"))
	hub.SendTo("binTest", frame)

	// Read the binary frame from the client side
	readCtx2, readCancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	msgType2, data, err := clientConn.Read(readCtx2)
	readCancel2()
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if msgType2 != websocket.MessageBinary {
		t.Fatalf("expected MessageBinary, got %v", msgType2)
	}
	if !IsBinaryVoiceFrame(data) {
		t.Fatal("data should be a binary voice frame")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not finish")
	}
}

// ---------------------------------------------------------------------------
// TestLegacyVoiceMessageInJSON verifies that old base64 JSON voice messages
// still work (backward compatibility).
// ---------------------------------------------------------------------------

func TestLegacyVoiceMessageInJSON(t *testing.T) {
	t.Parallel()

	// A message with voiceData should still decode correctly
	jsonData := []byte(`{
		"type": "chat",
		"from": "alice",
		"to": "broadcast",
		"text": "🎤 Голосовое сообщение (10с)",
		"ts": 1234567890,
		"voiceData": "dGVzdCBhdWRpbyBkYXRh",
		"voiceDuration": 10
	}`)

	msg, err := DecodeMessage(jsonData)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}
	if msg.VoiceData != "dGVzdCBhdWRpbyBkYXRh" {
		t.Fatalf("voiceData mismatch: %s", msg.VoiceData)
	}
	if msg.VoiceDuration != 10 {
		t.Fatalf("voiceDuration mismatch: %d", msg.VoiceDuration)
	}
}
