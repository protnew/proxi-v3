package chat

import (
	"errors"
	"sync"
	"time"
)

// PinnedMessage represents a message that has been pinned in a channel.
type PinnedMessage struct {
	MsgID     string `json:"msgID"`     // ID of the pinned message
	ChannelID string `json:"channelID"` // channel/conversation the pin belongs to
	PinnedBy  string `json:"pinnedBy"`  // userID of whoever pinned the message
	PinnedAt  int64  `json:"pinnedAt"`  // unix timestamp when the message was pinned
}

// ErrPinnedNotFound is returned when a requested pin does not exist.
var ErrPinnedNotFound = errors.New("pinned message not found")

// pinKey is a composite key uniquely identifying a pinned message within a
// channel (a message can be pinned in at most one channel).
type pinKey struct {
	channelID string
	msgID     string
}

// PinManager tracks pinned messages. It is safe for concurrent use.
type PinManager struct {
	mu    sync.RWMutex
	pins  map[pinKey]*PinnedMessage            // composite key → pin
	byMsg map[string]*PinnedMessage            // msgID → pin (fast lookup)
	byCh  map[string]map[string]*PinnedMessage // channelID → (msgID → pin)
}

// NewPinManager creates a new empty PinManager.
func NewPinManager() *PinManager {
	return &PinManager{
		pins:  make(map[pinKey]*PinnedMessage),
		byMsg: make(map[string]*PinnedMessage),
		byCh:  make(map[string]map[string]*PinnedMessage),
	}
}

// Pin marks a message as pinned in the given channel by the given user.
// Pinning an already-pinned message is safe (idempotent): it returns the
// existing pin record without error and without updating the PinnedBy/PinnedAt
// metadata. This protects against duplicate pins.
func (pm *PinManager) Pin(msgID, channelID, pinnedBy string) *PinnedMessage {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	key := pinKey{channelID: channelID, msgID: msgID}
	if existing, ok := pm.pins[key]; ok {
		// Duplicate pin — return the existing record unchanged (idempotent).
		return existing
	}

	pm_ := &PinnedMessage{
		MsgID:     msgID,
		ChannelID: channelID,
		PinnedBy:  pinnedBy,
		PinnedAt:  time.Now().Unix(),
	}
	pm.pins[key] = pm_

	// Index by message ID.
	pm.byMsg[msgID] = pm_

	// Index by channel.
	chMap, ok := pm.byCh[channelID]
	if !ok {
		chMap = make(map[string]*PinnedMessage)
		pm.byCh[channelID] = chMap
	}
	chMap[msgID] = pm_

	return pm_
}

// Unpin removes a pinned message. If the message was not pinned it returns
// ErrPinnedNotFound.
func (pm *PinManager) Unpin(msgID, channelID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	key := pinKey{channelID: channelID, msgID: msgID}
	if _, ok := pm.pins[key]; !ok {
		return ErrPinnedNotFound
	}

	delete(pm.pins, key)
	delete(pm.byMsg, msgID)

	if chMap, ok := pm.byCh[channelID]; ok {
		delete(chMap, msgID)
		if len(chMap) == 0 {
			delete(pm.byCh, channelID)
		}
	}
	return nil
}

// GetPinned returns the PinnedMessage record for the given message ID, or
// ErrPinnedNotFound if the message is not pinned.
func (pm *PinManager) GetPinned(msgID string) (*PinnedMessage, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	p, ok := pm.byMsg[msgID]
	if !ok {
		return nil, ErrPinnedNotFound
	}
	return p, nil
}

// ListPinned returns all pinned messages for a channel, ordered oldest-pin-first.
// Returns an empty slice if the channel has no pins.
func (pm *PinManager) ListPinned(channelID string) []*PinnedMessage {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	chMap, ok := pm.byCh[channelID]
	if !ok {
		return []*PinnedMessage{}
	}

	result := make([]*PinnedMessage, 0, len(chMap))
	for _, p := range chMap {
		result = append(result, p)
	}
	// Stable ordering by PinnedAt then MsgID.
	sortPins(result)
	return result
}

// Count returns the total number of pinned messages across all channels.
func (pm *PinManager) Count() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.pins)
}

// IsPinned reports whether a message is currently pinned.
func (pm *PinManager) IsPinned(msgID string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	_, ok := pm.byMsg[msgID]
	return ok
}

// sortPins sorts a slice of pinned messages by PinnedAt (asc) then MsgID.
// Uses insertion sort — pin counts are small.
func sortPins(pins []*PinnedMessage) {
	for i := 1; i < len(pins); i++ {
		for j := i; j > 0; j-- {
			a, b := pins[j-1], pins[j]
			swap := false
			if a.PinnedAt > b.PinnedAt {
				swap = true
			} else if a.PinnedAt == b.PinnedAt && a.MsgID > b.MsgID {
				swap = true
			}
			if !swap {
				break
			}
			pins[j-1], pins[j] = b, a
		}
	}
}
