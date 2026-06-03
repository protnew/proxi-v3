// Package stream provides live video streaming management via WebSocket binary frames.
//
// Streamers create streams, send binary frames through existing WS connections,
// and subscribers receive those frames in real-time.
package stream

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// Stream represents an active live stream.
type Stream struct {
	ID          string `json:"id"`
	ChannelName string `json:"channelName"`
	StreamerID  string `json:"streamerId"`
	ViewerCount int    `json:"viewerCount"`
	StartedAt   int64  `json:"startedAt"`
	Active      bool   `json:"active"`
}

// StreamFrame represents a binary video frame sent over WebSocket.
// Wire format (binary): [0x03] [4-byte stream ID length] [stream ID] [4-byte frame number] [payload]
const BinaryFrameStream byte = 0x03

// Manager manages active streams in memory.
type Manager struct {
	mu      sync.RWMutex
	streams map[string]*Stream // streamID → Stream
	// subscribers maps streamID to set of subscriber userIDs
	subscribers map[string]map[string]bool
}

// NewManager creates a new stream manager.
func NewManager() *Manager {
	return &Manager{
		streams:     make(map[string]*Stream),
		subscribers: make(map[string]map[string]bool),
	}
}

// CreateStream creates a new live stream and returns its ID.
func (m *Manager) CreateStream(channelName, streamerID string) *Stream {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateStreamID()
	s := &Stream{
		ID:          id,
		ChannelName: channelName,
		StreamerID:  streamerID,
		ViewerCount: 0,
		StartedAt:   time.Now().Unix(),
		Active:      true,
	}
	m.streams[id] = s
	m.subscribers[id] = map[string]bool{streamerID: true}
	return s
}

// EndStream marks a stream as ended and cleans up.
func (m *Manager) EndStream(streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.streams[streamID]
	if !ok {
		return fmt.Errorf("stream %s not found", streamID)
	}
	s.Active = false
	delete(m.streams, streamID)
	delete(m.subscribers, streamID)
	return nil
}

// ListStreams returns all active streams.
func (m *Manager) ListStreams() []Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Stream, 0, len(m.streams))
	for _, s := range m.streams {
		if s.Active {
			result = append(result, *s)
		}
	}
	return result
}

// GetStream returns a single stream by ID.
func (m *Manager) GetStream(streamID string) *Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.streams[streamID]
}

// Subscribe adds a viewer to a stream.
func (m *Manager) Subscribe(streamID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.streams[streamID]
	if !ok {
		return fmt.Errorf("stream %s not found", streamID)
	}
	if _, ok := m.subscribers[streamID]; !ok {
		m.subscribers[streamID] = map[string]bool{}
	}
	if !m.subscribers[streamID][userID] {
		m.subscribers[streamID][userID] = true
		s.ViewerCount++
	}
	return nil
}

// Unsubscribe removes a viewer from a stream.
func (m *Manager) Unsubscribe(streamID, userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.streams[streamID]
	if !ok {
		return
	}
	if subs, ok := m.subscribers[streamID]; ok {
		if subs[userID] && userID != s.StreamerID {
			delete(subs, userID)
			s.ViewerCount--
		}
	}
}

// GetSubscribers returns the list of subscriber userIDs for a stream.
func (m *Manager) GetSubscribers(streamID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs, ok := m.subscribers[streamID]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(subs))
	for uid := range subs {
		result = append(result, uid)
	}
	return result
}

// IsStreamFrame checks if a binary frame is a stream frame.
func IsStreamFrame(data []byte) bool {
	return len(data) > 0 && data[0] == BinaryFrameStream
}

// DecodeStreamFrame parses a binary stream frame.
// Returns: streamID, frameNumber, payload
func DecodeStreamFrame(data []byte) (streamID string, frameNumber uint32, payload []byte, err error) {
	if len(data) < 1 || data[0] != BinaryFrameStream {
		return "", 0, nil, fmt.Errorf("not a stream frame")
	}
	if len(data) < 5 {
		return "", 0, nil, fmt.Errorf("stream frame too short for ID length")
	}

	// 4-byte stream ID length (big-endian)
	idLen := uint32(data[1])<<24 | uint32(data[2])<<16 | uint32(data[3])<<8 | uint32(data[4])
	if len(data) < int(5+idLen+4) {
		return "", 0, nil, fmt.Errorf("stream frame too short for stream ID")
	}

	streamID = string(data[5 : 5+idLen])
	offset := 5 + idLen

	// 4-byte frame number (big-endian)
	frameNumber = uint32(data[offset])<<24 | uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3])
	offset += 4

	payload = data[offset:]
	return streamID, frameNumber, payload, nil
}

// EncodeStreamFrame builds a binary stream frame.
func EncodeStreamFrame(streamID string, frameNumber uint32, payload []byte) ([]byte, error) {
	idBytes := []byte(streamID)
	idLen := len(idBytes)

	buf := make([]byte, 0, 1+4+idLen+4+len(payload))
	buf = append(buf, BinaryFrameStream)
	buf = append(buf, byte(idLen>>24), byte(idLen>>16), byte(idLen>>8), byte(idLen))
	buf = append(buf, idBytes...)
	buf = append(buf, byte(frameNumber>>24), byte(frameNumber>>16), byte(frameNumber>>8), byte(frameNumber))
	buf = append(buf, payload...)
	return buf, nil
}

func generateStreamID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("str-%x", b)
}
