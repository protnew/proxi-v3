package chat

import (
	"sync"
	"time"
)

// slowChannel tracks the slow-mode configuration and per-user send timestamps
// for a single channel.
type slowChannel struct {
	interval    time.Duration                       // required gap between messages per user
	lastSend    map[string]time.Time                // userID → last message time
}

// SlowModeManager enforces per-channel slow-mode rate limiting. Each channel
// can have an independent interval; users must wait <interval> between
// consecutive messages in the same channel. Admins are exempt.
// Safe for concurrent use.
type SlowModeManager struct {
	mu       sync.RWMutex
	channels map[string]*slowChannel
	admins   map[string]bool // exempt users
}

// NewSlowModeManager creates a new SlowModeManager with no active slow modes.
func NewSlowModeManager() *SlowModeManager {
	return &SlowModeManager{
		channels: make(map[string]*slowChannel),
		admins:   make(map[string]bool),
	}
}

// SetSlowMode configures the required interval between messages for a channel.
// A zero or negative interval disables slow mode for the channel.
func (sm *SlowModeManager) SetSlowMode(channelID string, interval time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if interval <= 0 {
		delete(sm.channels, channelID)
		return
	}

	ch, ok := sm.channels[channelID]
	if !ok {
		ch = &slowChannel{
			interval: interval,
			lastSend: make(map[string]time.Time),
		}
		sm.channels[channelID] = ch
	} else {
		ch.interval = interval
	}
}

// GetSlowMode returns the current slow-mode interval for a channel, or 0 if
// slow mode is not enabled.
func (sm *SlowModeManager) GetSlowMode(channelID string) time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if ch, ok := sm.channels[channelID]; ok {
		return ch.interval
	}
	return 0
}

// SetAdmin marks (or unmarks) a user as exempt from slow-mode restrictions.
func (sm *SlowModeManager) SetAdmin(userID string, isAdmin bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if isAdmin {
		sm.admins[userID] = true
	} else {
		delete(sm.admins, userID)
	}
}

// IsAdmin reports whether a user is exempt from slow mode.
func (sm *SlowModeManager) IsAdmin(userID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.admins[userID]
}

// CanSend reports whether the given user is allowed to send a message in the
// given channel right now. It returns true when:
//   - the user is an admin (exempt), or
//   - the channel has no slow mode, or
//   - enough time has elapsed since the user's last message in the channel.
func (sm *SlowModeManager) CanSend(userID, channelID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Admins are always allowed.
	if sm.admins[userID] {
		return true
	}

	ch, ok := sm.channels[channelID]
	if !ok {
		return true // no slow mode on this channel
	}

	last, hasLast := ch.lastSend[userID]
	if !hasLast {
		return true // first message in channel
	}
	return time.Since(last) >= ch.interval
}

// TimeUntilCanSend returns how long the user must wait before sending in the
// channel. Returns 0 if the user can send now. Admins always return 0.
func (sm *SlowModeManager) TimeUntilCanSend(userID, channelID string) time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.admins[userID] {
		return 0
	}
	ch, ok := sm.channels[channelID]
	if !ok {
		return 0
	}
	last, hasLast := ch.lastSend[userID]
	if !hasLast {
		return 0
	}
	elapsed := time.Since(last)
	if elapsed >= ch.interval {
		return 0
	}
	return ch.interval - elapsed
}

// RecordSend must be called after a message is successfully sent to update the
// user's last-send timestamp for the channel. This is what enforces the rate
// limit on subsequent CanSend checks.
func (sm *SlowModeManager) RecordSend(userID, channelID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ch, ok := sm.channels[channelID]
	if !ok {
		return // no slow mode — nothing to record
	}
	ch.lastSend[userID] = time.Now()
}

// ResetUser clears the send history for a user in a channel, allowing them to
// send immediately. Useful for tests and admin overrides.
func (sm *SlowModeManager) ResetUser(userID, channelID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if ch, ok := sm.channels[channelID]; ok {
		delete(ch.lastSend, userID)
	}
}
