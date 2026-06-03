package stream

import (
	"bytes"
	"sync"
	"testing"
)

// --- CreateStream ---

func TestCreateStream(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("gaming", "user1")

	if s == nil {
		t.Fatal("expected non-nil stream")
	}
	if s.ID == "" {
		t.Error("expected non-empty stream ID")
	}
	if s.ChannelName != "gaming" {
		t.Errorf("expected channelName=gaming, got %s", s.ChannelName)
	}
	if s.StreamerID != "user1" {
		t.Errorf("expected streamerID=user1, got %s", s.StreamerID)
	}
	if s.ViewerCount != 0 {
		t.Errorf("expected viewerCount=0, got %d", s.ViewerCount)
	}
	if !s.Active {
		t.Error("expected stream to be active")
	}
	if s.StartedAt <= 0 {
		t.Error("expected positive StartedAt timestamp")
	}

	// Streamer should be auto-subscribed
	subs := m.GetSubscribers(s.ID)
	if len(subs) != 1 || subs[0] != "user1" {
		t.Errorf("expected streamer to be auto-subscribed, got %v", subs)
	}
}

func TestCreateStream_MultipleUniqueIDs(t *testing.T) {
	m := NewManager()
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s := m.CreateStream("ch", "streamer")
		if ids[s.ID] {
			t.Fatalf("duplicate stream ID: %s", s.ID)
		}
		ids[s.ID] = true
	}
}

// --- EndStream ---

func TestEndStream(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("music", "user2")

	err := m.EndStream(s.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stream should be removed from map
	got := m.GetStream(s.ID)
	if got != nil {
		t.Error("expected stream to be removed after EndStream")
	}

	// Subscribers should be cleaned up
	subs := m.GetSubscribers(s.ID)
	if subs != nil {
		t.Errorf("expected nil subscribers after end, got %v", subs)
	}
}

func TestEndStream_NotFound(t *testing.T) {
	m := NewManager()
	err := m.EndStream("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent stream")
	}
}

// --- ListStreams ---

func TestListStreams(t *testing.T) {
	m := NewManager()
	s1 := m.CreateStream("ch1", "u1")
	s2 := m.CreateStream("ch2", "u2")

	list := m.ListStreams()
	if len(list) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(list))
	}

	found := map[string]bool{s1.ID: false, s2.ID: false}
	for _, s := range list {
		found[s.ID] = true
		if !s.Active {
			t.Error("expected listed stream to be active")
		}
	}
	for id, ok := range found {
		if !ok {
			t.Errorf("stream %s not found in list", id)
		}
	}
}

func TestListStreams_Empty(t *testing.T) {
	m := NewManager()
	list := m.ListStreams()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestListStreams_AfterEnd(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("ch", "u1")
	m.CreateStream("ch2", "u2")

	m.EndStream(s.ID)
	list := m.ListStreams()
	if len(list) != 1 {
		t.Fatalf("expected 1 stream after ending one, got %d", len(list))
	}
	if list[0].ID == s.ID {
		t.Error("ended stream should not appear in list")
	}
}

// --- GetStream ---

func TestGetStream(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("test", "u1")

	got := m.GetStream(s.ID)
	if got == nil {
		t.Fatal("expected stream, got nil")
	}
	if got.ID != s.ID {
		t.Errorf("expected ID=%s, got %s", s.ID, got.ID)
	}
	if got.ChannelName != "test" {
		t.Errorf("expected channelName=test, got %s", got.ChannelName)
	}
}

func TestGetStream_NotFound(t *testing.T) {
	m := NewManager()
	got := m.GetStream("nope")
	if got != nil {
		t.Error("expected nil for nonexistent stream")
	}
}

// --- Subscribe ---

func TestSubscribe(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("ch", "streamer")

	err := m.Subscribe(s.ID, "viewer1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	subs := m.GetSubscribers(s.ID)
	if len(subs) != 2 { // streamer + viewer1
		t.Errorf("expected 2 subscribers, got %d: %v", len(subs), subs)
	}

	got := m.GetStream(s.ID)
	if got.ViewerCount != 1 {
		t.Errorf("expected ViewerCount=1, got %d", got.ViewerCount)
	}
}

func TestSubscribe_StreamNotFound(t *testing.T) {
	m := NewManager()
	err := m.Subscribe("nonexistent", "user")
	if err == nil {
		t.Error("expected error for subscribing to nonexistent stream")
	}
}

func TestSubscribe_DuplicateNoDoubleCount(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("ch", "streamer")

	m.Subscribe(s.ID, "viewer1")
	m.Subscribe(s.ID, "viewer1") // duplicate

	got := m.GetStream(s.ID)
	if got.ViewerCount != 1 {
		t.Errorf("expected ViewerCount=1 after duplicate subscribe, got %d", got.ViewerCount)
	}
}

// --- Unsubscribe ---

func TestUnsubscribe(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("ch", "streamer")

	m.Subscribe(s.ID, "viewer1")
	m.Unsubscribe(s.ID, "viewer1")

	got := m.GetStream(s.ID)
	if got.ViewerCount != 0 {
		t.Errorf("expected ViewerCount=0 after unsubscribe, got %d", got.ViewerCount)
	}

	subs := m.GetSubscribers(s.ID)
	for _, uid := range subs {
		if uid == "viewer1" {
			t.Error("viewer1 should have been removed from subscribers")
		}
	}
}

func TestUnsubscribe_StreamerCannotUnsubscribe(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("ch", "streamer")

	m.Unsubscribe(s.ID, "streamer")

	got := m.GetStream(s.ID)
	if got == nil {
		t.Fatal("stream should still exist")
	}
	// Streamer is auto-subscribed, cannot unsubscribe
	subs := m.GetSubscribers(s.ID)
	found := false
	for _, uid := range subs {
		if uid == "streamer" {
			found = true
		}
	}
	if !found {
		t.Error("streamer should still be subscribed")
	}
}

func TestUnsubscribe_NonexistentStream(t *testing.T) {
	m := NewManager()
	// Should not panic
	m.Unsubscribe("nonexistent", "user")
}

func TestUnsubscribe_NonexistentUser(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("ch", "streamer")

	// Unsubscribing a user who never subscribed should not panic
	m.Unsubscribe(s.ID, "ghost")

	got := m.GetStream(s.ID)
	if got.ViewerCount != 0 {
		t.Errorf("expected ViewerCount=0, got %d", got.ViewerCount)
	}
}

// --- GetSubscribers ---

func TestGetSubscribers(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("ch", "streamer")

	m.Subscribe(s.ID, "v1")
	m.Subscribe(s.ID, "v2")

	subs := m.GetSubscribers(s.ID)
	if len(subs) != 3 { // streamer + v1 + v2
		t.Errorf("expected 3 subscribers, got %d: %v", len(subs), subs)
	}
}

func TestGetSubscribers_NoStream(t *testing.T) {
	m := NewManager()
	subs := m.GetSubscribers("nonexistent")
	if subs != nil {
		t.Errorf("expected nil, got %v", subs)
	}
}

// --- IsStreamFrame ---

func TestIsStreamFrame_Valid(t *testing.T) {
	data := []byte{BinaryFrameStream, 0x00, 0x01, 0x02}
	if !IsStreamFrame(data) {
		t.Error("expected true for valid stream frame")
	}
}

func TestIsStreamFrame_InvalidPrefix(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	if IsStreamFrame(data) {
		t.Error("expected false for non-stream frame")
	}
}

func TestIsStreamFrame_Empty(t *testing.T) {
	if IsStreamFrame(nil) {
		t.Error("expected false for nil data")
	}
	if IsStreamFrame([]byte{}) {
		t.Error("expected false for empty data")
	}
}

// --- EncodeStreamFrame / DecodeStreamFrame round-trip ---

func TestEncodeDecodeRoundTrip(t *testing.T) {
	streamID := "str-abcdef1234567890"
	frameNumber := uint32(42)
	payload := []byte("hello video frame data")

	encoded, err := EncodeStreamFrame(streamID, frameNumber, payload)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	if !IsStreamFrame(encoded) {
		t.Error("encoded frame should be recognized as stream frame")
	}

	gotID, gotFrame, gotPayload, err := DecodeStreamFrame(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if gotID != streamID {
		t.Errorf("expected streamID=%s, got %s", streamID, gotID)
	}
	if gotFrame != frameNumber {
		t.Errorf("expected frameNumber=%d, got %d", frameNumber, gotFrame)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("expected payload=%q, got %q", payload, gotPayload)
	}
}

func TestEncodeDecode_EmptyPayload(t *testing.T) {
	encoded, err := EncodeStreamFrame("sid", 0, nil)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	gotID, gotFrame, gotPayload, err := DecodeStreamFrame(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if gotID != "sid" {
		t.Errorf("expected sid, got %s", gotID)
	}
	if gotFrame != 0 {
		t.Errorf("expected frame 0, got %d", gotFrame)
	}
	if len(gotPayload) != 0 {
		t.Errorf("expected empty payload, got %q", gotPayload)
	}
}

func TestEncodeDecode_LargeFrameNumber(t *testing.T) {
	encoded, _ := EncodeStreamFrame("test", 0xFFFFFFFF, []byte("x"))
	gotID, gotFrame, gotPayload, err := DecodeStreamFrame(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if gotID != "test" {
		t.Errorf("expected test, got %s", gotID)
	}
	if gotFrame != 0xFFFFFFFF {
		t.Errorf("expected 0xFFFFFFFF, got %d", gotFrame)
	}
	if !bytes.Equal(gotPayload, []byte("x")) {
		t.Errorf("payload mismatch")
	}
}

// --- DecodeStreamFrame error cases ---

func TestDecodeStreamFrame_NotAStreamFrame(t *testing.T) {
	_, _, _, err := DecodeStreamFrame([]byte{0x01, 0x02})
	if err == nil {
		t.Error("expected error for non-stream frame")
	}
}

func TestDecodeStreamFrame_Empty(t *testing.T) {
	_, _, _, err := DecodeStreamFrame(nil)
	if err == nil {
		t.Error("expected error for nil data")
	}
}

func TestDecodeStreamFrame_TooShortForIDLength(t *testing.T) {
	// Only the prefix byte, no 4-byte length
	_, _, _, err := DecodeStreamFrame([]byte{BinaryFrameStream, 0x00, 0x01})
	if err == nil {
		t.Error("expected error for frame too short for ID length")
	}
}

func TestDecodeStreamFrame_TooShortForStreamID(t *testing.T) {
	// Prefix + 4-byte length saying ID is 10 bytes, but no actual ID or frame number
	data := []byte{BinaryFrameStream, 0x00, 0x00, 0x00, 0x0A} // idLen=10, but no data
	_, _, _, err := DecodeStreamFrame(data)
	if err == nil {
		t.Error("expected error for frame too short for stream ID")
	}
}

func TestDecodeStreamFrame_TooShortForFrameNumber(t *testing.T) {
	// Prefix + idLen=3 + 3-byte ID + but missing 4-byte frame number
	data := []byte{BinaryFrameStream, 0x00, 0x00, 0x00, 0x03, 'a', 'b', 'c', 0x00}
	_, _, _, err := DecodeStreamFrame(data)
	if err == nil {
		t.Error("expected error for frame too short for frame number")
	}
}

// --- Concurrent tests ---

func TestConcurrentCreateEnd(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup
	const n = 50

	ids := make(chan string, n)

	// Concurrent creates
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := m.CreateStream("ch", "streamer")
			ids <- s.ID
		}(i)
	}
	wg.Wait()
	close(ids)

	// All should be created
	list := m.ListStreams()
	if len(list) != n {
		t.Errorf("expected %d streams, got %d", n, len(list))
	}

	// Concurrent ends
	var wg2 sync.WaitGroup
	for id := range ids {
		wg2.Add(1)
		go func(id string) {
			defer wg2.Done()
			err := m.EndStream(id)
			if err != nil {
				t.Errorf("unexpected error ending stream %s: %v", id, err)
			}
		}(id)
	}
	wg2.Wait()

	// All should be gone
	list = m.ListStreams()
	if len(list) != 0 {
		t.Errorf("expected 0 streams after ending all, got %d", len(list))
	}
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("ch", "streamer")

	const viewers = 100
	var wg sync.WaitGroup

	// Concurrent subscribes
	for i := 0; i < viewers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			userID := "viewer"
			_ = m.Subscribe(s.ID, userID) // same user subscribing many times
		}(i)
	}
	wg.Wait()

	got := m.GetStream(s.ID)
	if got.ViewerCount != 1 {
		t.Errorf("expected ViewerCount=1 after concurrent duplicate subscribes, got %d", got.ViewerCount)
	}

	// Concurrent unsubscribes
	for i := 0; i < viewers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Unsubscribe(s.ID, "viewer")
		}()
	}
	wg.Wait()

	got = m.GetStream(s.ID)
	if got.ViewerCount != 0 {
		t.Errorf("expected ViewerCount=0 after unsubscribe, got %d", got.ViewerCount)
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	m := NewManager()
	s := m.CreateStream("ch", "streamer")

	const ops = 200
	var wg sync.WaitGroup

	for i := 0; i < ops; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 5 {
			case 0:
				m.Subscribe(s.ID, "viewer")
			case 1:
				m.Unsubscribe(s.ID, "viewer")
			case 2:
				m.GetStream(s.ID)
			case 3:
				m.ListStreams()
			case 4:
				m.GetSubscribers(s.ID)
			}
		}(i)
	}
	wg.Wait()
	// Should not panic or deadlock
}

func TestConcurrentEncodeDecode(t *testing.T) {
	const ops = 100
	var wg sync.WaitGroup

	for i := 0; i < ops; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload := []byte{byte(i), byte(i >> 8)}
			frameNum := uint32(i)
			streamID := "stream-concurrent"

			encoded, err := EncodeStreamFrame(streamID, frameNum, payload)
			if err != nil {
				t.Errorf("encode error: %v", err)
				return
			}

			if !IsStreamFrame(encoded) {
				t.Error("encoded should be a stream frame")
				return
			}

			gotID, gotFrame, gotPayload, err := DecodeStreamFrame(encoded)
			if err != nil {
				t.Errorf("decode error: %v", err)
				return
			}
			if gotID != streamID || gotFrame != frameNum || !bytes.Equal(gotPayload, payload) {
				t.Errorf("round-trip mismatch: id=%s frame=%d payload=%v", gotID, gotFrame, gotPayload)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentManyStreamsManySubscribers(t *testing.T) {
	m := NewManager()
	const numStreams = 20
	const numViewers = 10

	streamIDs := make([]string, numStreams)
	for i := 0; i < numStreams; i++ {
		s := m.CreateStream("ch", "streamer")
		streamIDs[i] = s.ID
	}

	var wg sync.WaitGroup
	for si := 0; si < numStreams; si++ {
		for vi := 0; vi < numViewers; vi++ {
			wg.Add(1)
			go func(streamID string, viewerID string) {
				defer wg.Done()
				_ = m.Subscribe(streamID, viewerID)
			}(streamIDs[si], "viewer")
		}
	}
	wg.Wait()

	// Each stream should have 2 subscribers: streamer + "viewer" (all same ID = 1 unique viewer)
	for _, sid := range streamIDs {
		got := m.GetStream(sid)
		if got.ViewerCount != 1 {
			t.Errorf("stream %s: expected ViewerCount=1, got %d", sid, got.ViewerCount)
		}
		subs := m.GetSubscribers(sid)
		if len(subs) != 2 {
			t.Errorf("stream %s: expected 2 subscribers, got %d", sid, len(subs))
		}
	}
}
