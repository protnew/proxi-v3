package content

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestStreamSession_Next(t *testing.T) {
	// Create data large enough for 3 segments at 10s/seg
	segBytes := BytesPerSecond * 10 // 10s segment
	data := make([]byte, segBytes*2+12345)
	for i := range data {
		data[i] = byte(i % 256)
	}

	ss := NewStreamSession("content-1", data, 10*time.Second)

	if ss.ContentID != "content-1" {
		t.Errorf("ContentID = %q, want %q", ss.ContentID, "content-1")
	}
	if len(ss.Segments) != 3 {
		t.Fatalf("Segments count = %d, want 3", len(ss.Segments))
	}

	// Iterate all segments
	count := 0
	for {
		seg, err := ss.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextSegment: %v", err)
		}
		if seg.Index != count {
			t.Errorf("segment Index = %d, want %d", seg.Index, count)
		}
		if len(seg.Data) == 0 {
			t.Errorf("segment %d has empty data", count)
		}
		count++
	}
	if count != 3 {
		t.Errorf("iterated %d segments, want 3", count)
	}

	// Call again after EOF
	_, err := ss.NextSegment()
	if err != io.EOF {
		t.Errorf("after EOF, error = %v, want io.EOF", err)
	}
}

func TestStreamSession_Seek(t *testing.T) {
	segBytes := BytesPerSecond * 10
	data := make([]byte, segBytes*10)
	ss := NewStreamSession("content-2", data, 10*time.Second)

	if len(ss.Segments) != 10 {
		t.Fatalf("Segments count = %d, want 10", len(ss.Segments))
	}

	// Seek to segment 5
	if err := ss.Seek(5); err != nil {
		t.Fatalf("Seek(5): %v", err)
	}
	if ss.CurrentSeg != 5 {
		t.Errorf("CurrentSeg after seek = %d, want 5", ss.CurrentSeg)
	}

	// Next should return segment 5
	seg, err := ss.NextSegment()
	if err != nil {
		t.Fatalf("NextSegment after seek: %v", err)
	}
	if seg.Index != 5 {
		t.Errorf("segment Index = %d, want 5", seg.Index)
	}

	// Seek to invalid index
	if err := ss.Seek(-1); err == nil {
		t.Error("Seek(-1) should return error")
	}
	if err := ss.Seek(100); err == nil {
		t.Error("Seek(100) should return error")
	}
}

func TestStreamSession_Playlist(t *testing.T) {
	segBytes := BytesPerSecond * 10
	data := make([]byte, segBytes*3)
	ss := NewStreamSession("content-3", data, 10*time.Second)

	// Playlist from start
	playlist := ss.GeneratePlaylist()
	checks := []string{
		"#EXTM3U",
		"#EXT-X-VERSION:3",
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXTINF:10.000000,",
		"segment_000000.ts",
		"segment_000001.ts",
		"segment_000002.ts",
		"#EXT-X-ENDLIST",
	}
	for _, want := range checks {
		if !strings.Contains(playlist, want) {
			t.Errorf("playlist missing %q\nfull:\n%s", want, playlist)
		}
	}

	// Advance to segment 2 and regenerate
	ss.NextSegment()
	ss.NextSegment()
	playlist2 := ss.GeneratePlaylist()
	if !strings.Contains(playlist2, "#EXT-X-MEDIA-SEQUENCE:2") {
		t.Errorf("playlist after advance missing SEQUENCE:2\nfull:\n%s", playlist2)
	}
	if strings.Contains(playlist2, "segment_000000.ts") {
		t.Error("playlist after advance should not contain segment_000000.ts")
	}
	if !strings.Contains(playlist2, "segment_000002.ts") {
		t.Error("playlist after advance should contain segment_000002.ts")
	}
}

func TestStreamSession_EmptyData(t *testing.T) {
	ss := NewStreamSession("empty", nil, 10*time.Second)
	if len(ss.Segments) != 0 {
		t.Errorf("Segments count = %d, want 0", len(ss.Segments))
	}

	seg, err := ss.NextSegment()
	if err != io.EOF {
		t.Errorf("NextSegment on empty: seg=%v, err=%v, want io.EOF", seg, err)
	}

	// Playlist on empty
	playlist := ss.GeneratePlaylist()
	if !strings.Contains(playlist, "#EXTM3U") {
		t.Error("empty playlist should still contain #EXTM3U")
	}
	if !strings.Contains(playlist, "#EXT-X-ENDLIST") {
		t.Error("empty playlist should contain #EXT-X-ENDLIST")
	}

	// Seek on empty
	if err := ss.Seek(0); err == nil {
		t.Error("Seek(0) on empty session should return error")
	}

	// Also test with empty byte slice
	ss2 := NewStreamSession("empty2", []byte{}, 10*time.Second)
	_, err = ss2.NextSegment()
	if err != io.EOF {
		t.Errorf("NextSegment on empty slice: err=%v, want io.EOF", err)
	}
}
