package content

import (
	"fmt"
	"io"
	"strings"
)

// SegmentDuration is the default HLS segment duration in seconds.
const SegmentDuration = 10

// BytesPerSecond approximates 125 KB/s bitrate for byte-to-time conversion.
const BytesPerSecond = 125 * 1024

// Segment represents one HLS segment (.ts chunk).
type Segment struct {
	Index    int
	Duration float64 // seconds
	Data     []byte
}

// SegmentReader wraps an io.Reader and produces HLS segments based on
// byte-level timing at approximately 125 KB/s. Each segment targets
// SegmentDuration seconds of playback.
type SegmentReader struct {
	reader      io.Reader
	index       int
	segmentSize int // bytes per segment = SegmentDuration * BytesPerSecond
	done        bool
}

// NewSegmentReader creates a SegmentReader from any io.Reader.
func NewSegmentReader(r io.Reader) *SegmentReader {
	return &SegmentReader{
		reader:      r,
		segmentSize: SegmentDuration * BytesPerSecond,
	}
}

// Next reads the next segment from the underlying reader. Returns io.EOF
// when there is no more data. The last segment may be shorter and have a
// proportionally smaller Duration.
func (sr *SegmentReader) Next() (*Segment, error) {
	if sr.done {
		return nil, io.EOF
	}
	buf := make([]byte, sr.segmentSize)
	n, err := io.ReadFull(sr.reader, buf)
	if n == 0 {
		sr.done = true
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("segment read: %w", err)
	}

	data := make([]byte, n)
	copy(data, buf[:n])

	duration := float64(n) / float64(BytesPerSecond)
	seg := &Segment{
		Index:    sr.index,
		Duration: duration,
		Data:     data,
	}
	sr.index++

	if err == io.EOF || err == io.ErrUnexpectedEOF {
		sr.done = true
	}
	return seg, nil
}

// GenerateM3U8 builds an HLS master/media playlist string from segments.
// The EXT-X-TARGETDURATION is set to SegmentDuration. The playlist is
// formatted as a standard HLS v3 media playlist.
func GenerateM3U8(segments []Segment) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:3\n")
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", SegmentDuration))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	for _, seg := range segments {
		b.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", seg.Duration))
		b.WriteString(fmt.Sprintf("segment_%06d.ts\n", seg.Index))
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}
