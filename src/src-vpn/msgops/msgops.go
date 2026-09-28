package msgops

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// EditedMessage stores edit history
type EditedMessage struct {
	ID        string `json:"id"`
	GroupID   string `json:"group_id,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	From      string `json:"from"`
	Text      string `json:"text"`
	PrevText  string `json:"prev_text"`
	EditedAt  int64  `json:"edited_at"`
	EditedN   int    `json:"edited_n"`
}

// DeletedMessage records a deletion
type DeletedMessage struct {
	ID        string `json:"id"`
	DeletedBy string `json:"deleted_by"`
	DeletedAt int64  `json:"deleted_at"`
}

// TypingStatus tracks who is typing where
type TypingStatus struct {
	UserID    string `json:"user_id"`
	RoomID    string `json:"room_id"`
	Timestamp int64  `json:"ts"`
}

// ScheduledMsg is a message queued for future delivery
type ScheduledMsg struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`        // roomID or userID
	Text      string `json:"text"`
	SendAt    int64  `json:"send_at"`
	CreatedAt int64  `json:"created_at"`
	Sent      bool   `json:"sent"`
}

// Manager handles message operations
type Manager struct {
	mu          sync.RWMutex
	edited      map[string]*EditedMessage
	deleted     map[string]*DeletedMessage
	typing      map[string]*TypingStatus
	scheduled   map[string]*ScheduledMsg
	subscribers []func(eventType string, payload []byte)
}

// NewManager creates a message ops manager
func NewManager() *Manager {
	return &Manager{
		edited:    make(map[string]*EditedMessage),
		deleted:   make(map[string]*DeletedMessage),
		typing:    make(map[string]*TypingStatus),
		scheduled: make(map[string]*ScheduledMsg),
	}
}

// EditMessage edits a message, stores prev version
func (m *Manager) EditMessage(msgID, userID, newText string) (*EditedMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, deleted := m.deleted[msgID]; deleted {
		return nil, ErrMessageDeleted
	}

	editN := 1
	prevText := ""
	if existing, ok := m.edited[msgID]; ok {
		prevText = existing.Text
		editN = existing.EditedN + 1
	}

	edit := &EditedMessage{
		ID:       msgID,
		From:     userID,
		Text:     newText,
		PrevText: prevText,
		EditedAt: time.Now().Unix(),
		EditedN:  editN,
	}
	m.edited[msgID] = edit

	m.emit("message_edited", edit)
	return edit, nil
}

// DeleteMessage marks a message as deleted
func (m *Manager) DeleteMessage(msgID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.deleted[msgID]; ok {
		return ErrAlreadyDeleted
	}

	del := &DeletedMessage{
		ID:        msgID,
		DeletedBy: userID,
		DeletedAt: time.Now().Unix(),
	}
	m.deleted[msgID] = del

	m.emit("message_deleted", del)
	return nil
}

// IsDeleted checks if a message was deleted
func (m *Manager) IsDeleted(msgID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.deleted[msgID]
	return ok
}

// GetEdit returns edit info for a message
func (m *Manager) GetEdit(msgID string) (*EditedMessage, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.edited[msgID]
	return e, ok
}

// SetTyping marks user as typing in a room
func (m *Manager) SetTyping(userID, roomID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := roomID + ":" + userID
	m.typing[key] = &TypingStatus{
		UserID:    userID,
		RoomID:    roomID,
		Timestamp: time.Now().Unix(),
	}

	m.emit("typing", map[string]interface{}{
		"user_id": userID,
		"room_id": roomID,
		"ts":      time.Now().Unix(),
	})
}

// GetTyping returns who is typing in a room (excluding userID)
func (m *Manager) GetTyping(roomID, excludeUserID string) []*TypingStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now().Unix()
	var result []*TypingStatus

	for key, ts := range m.typing {
		if ts.RoomID != roomID {
			continue
		}
		if now-ts.Timestamp > 5 {
			delete(m.typing, key)
			continue
		}
		if ts.UserID != excludeUserID {
			result = append(result, ts)
		}
	}
	return result
}

// ScheduleMessage queues a message for future delivery
func (m *Manager) ScheduleMessage(from, to, text string, sendAt int64) *ScheduledMsg {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("sched_%d", time.Now().UnixNano())
	s := &ScheduledMsg{
		ID:        id,
		From:      from,
		To:        to,
		Text:      text,
		SendAt:    sendAt,
		CreatedAt: time.Now().Unix(),
	}
	m.scheduled[id] = s
	return s
}

// GetPendingScheduled returns messages that should be sent now
func (m *Manager) GetPendingScheduled() []*ScheduledMsg {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()
	var result []*ScheduledMsg

	for _, s := range m.scheduled {
		if !s.Sent && s.SendAt <= now {
			s.Sent = true
			result = append(result, s)
		}
	}
	return result
}

// ListScheduled returns all pending scheduled messages for a user
func (m *Manager) ListScheduled(userID string) []*ScheduledMsg {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ScheduledMsg
	for _, s := range m.scheduled {
		if s.From == userID && !s.Sent {
			result = append(result, s)
		}
	}
	return result
}

// CancelScheduled removes a scheduled message
func (m *Manager) CancelScheduled(id, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.scheduled[id]
	if !ok {
		return ErrNotFound
	}
	if s.From != userID {
		return ErrNotOwner
	}
	if s.Sent {
		return ErrAlreadySent
	}
	delete(m.scheduled, id)
	return nil
}

// OnSubscribe registers an event listener
func (m *Manager) OnSubscribe(fn func(eventType string, payload []byte)) {
	m.subscribers = append(m.subscribers, fn)
}

func (m *Manager) emit(eventType string, data interface{}) {
	payload, _ := json.Marshal(data)
	for _, fn := range m.subscribers {
		go fn(eventType, payload)
	}
}

// Errors
var (
	ErrMessageDeleted = fmt.Errorf("message already deleted")
	ErrAlreadyDeleted = fmt.Errorf("message already deleted")
	ErrNotFound       = fmt.Errorf("scheduled message not found")
	ErrNotOwner       = fmt.Errorf("not your message")
	ErrAlreadySent    = fmt.Errorf("message already sent")
)

// Ensure log is used (for future logging)
var _ = log.Println
