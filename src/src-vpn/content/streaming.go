package content

import (
	"errors"
	"fmt"
	"io"
	"time"
)

// HLSSegment represents one HLS segment in a stream session.
type HLSSegment struct {
	Index    int
	Duration float64
	Data     []byte
}

// StreamSession manages sequential access to HLS segments for a single stream.
type StreamSession struct {
	ContentID  string
	Segments   []HLSSegment
	CurrentSeg int
}

// NewStreamSession splits data into segments of the given duration (based on
// ~125 KB/s bitrate) and returns a new StreamSession.
func NewStreamSession(contentID string, data []byte, segDuration time.Duration) *StreamSession {
	if len(data) == 0 {
		return &StreamSession{
			ContentID: contentID,
		}
	}

	segSize := int(float64(BytesPerSecond) * segDuration.Seconds())
	if segSize <= 0 {
		segSize = BytesPerSecond * 10 // default 10s segment
	}

	var segments []HLSSegment
	for offset := 0; offset < len(data); offset += segSize {
		end := offset + segSize
		if end > len(data) {
			end = len(data)
		}
		chunk := make([]byte, end-offset)
		copy(chunk, data[offset:end])

		duration := float64(len(chunk)) / float64(BytesPerSecond)
		segments = append(segments, HLSSegment{
			Index:    len(segments),
			Duration: duration,
			Data:     chunk,
		})
	}

	return &StreamSession{
		ContentID: contentID,
		Segments:  segments,
	}
}

// ErrEndOfStream is returned when no more segments are available.
var ErrEndOfStream = errors.New("end of stream")

// ErrInvalidSegment is returned when seeking to an invalid segment index.
var ErrInvalidSegment = errors.New("invalid segment index")

// NextSegment returns the next segment or io.EOF if the stream is exhausted.
func (s *StreamSession) NextSegment() (*HLSSegment, error) {
	if s.CurrentSeg >= len(s.Segments) {
		return nil, io.EOF
	}
	seg := &s.Segments[s.CurrentSeg]
	s.CurrentSeg++
	return seg, nil
}

// Seek jumps to the specified segment index.
func (s *StreamSession) Seek(segmentIndex int) error {
	if segmentIndex < 0 || segmentIndex >= len(s.Segments) {
		return ErrInvalidSegment
	}
	s.CurrentSeg = segmentIndex
	return nil
}

// GeneratePlaylist builds an M3U8 playlist string starting from the current
// position in the stream.
func (s *StreamSession) GeneratePlaylist() string {
	result := "#EXTM3U\n"
	result += "#EXT-X-VERSION:3\n"
	result += fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", SegmentDuration)
	result += fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d\n", s.CurrentSeg)

	for i := s.CurrentSeg; i < len(s.Segments); i++ {
		seg := s.Segments[i]
		result += fmt.Sprintf("#EXTINF:%.6f,\n", seg.Duration)
		result += fmt.Sprintf("segment_%06d.ts\n", seg.Index)
	}

	result += "#EXT-X-ENDLIST\n"
	return result
}
