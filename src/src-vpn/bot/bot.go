// Package bot provides a bot framework and sticker management for Unkillable Messenger.
package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Message represents a chat message passed to bot handlers.
type Message struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"`
}

// BotHandler is the interface that bots must implement to process messages.
type BotHandler interface {
	// OnMessage is called for every incoming message the bot is subscribed to.
	OnMessage(msg Message) error
	// Name returns the bot's display name.
	Name() string
}

// BotRegistration represents a registered bot.
type BotRegistration struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token,omitempty"`
	OwnerNpub string `json:"ownerNpub"`
	Command   string `json:"command"` // e.g. "/echo"
	CreatedAt int64  `json:"createdAt"`
	Active    bool   `json:"active"`
}

// Manager manages bots and sticker packs.
type Manager struct {
	mu   sync.RWMutex
	bots map[string]*BotRegistration // botID → registration
	// handlers maps botID to handler instance
	handlers map[string]BotHandler
	// commandIndex maps command string to botID
	commandIndex map[string]string
}

// NewManager creates a new bot manager.
func NewManager() *Manager {
	return &Manager{
		bots:         make(map[string]*BotRegistration),
		handlers:     make(map[string]BotHandler),
		commandIndex: make(map[string]string),
	}
}

// RegisterBot registers a new bot with its handler.
func (m *Manager) RegisterBot(name, ownerNpub, command string, handler BotHandler) *BotRegistration {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("bot-%d-%d", time.Now().UnixNano(), len(m.bots))
	reg := &BotRegistration{
		ID:        id,
		Name:      name,
		Token:     fmt.Sprintf("tk-%x", id),
		OwnerNpub: ownerNpub,
		Command:   command,
		CreatedAt: time.Now().Unix(),
		Active:    true,
	}
	m.bots[id] = reg
	m.handlers[id] = handler
	if command != "" {
		m.commandIndex[command] = id
	}
	return reg
}

// UnregisterBot removes a bot.
func (m *Manager) UnregisterBot(botID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reg, ok := m.bots[botID]
	if !ok {
		return fmt.Errorf("bot %s not found", botID)
	}
	if reg.Command != "" {
		delete(m.commandIndex, reg.Command)
	}
	delete(m.bots, botID)
	delete(m.handlers, botID)
	return nil
}

// ListBots returns all registered bots.
func (m *Manager) ListBots() []BotRegistration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]BotRegistration, 0, len(m.bots))
	for _, b := range m.bots {
		result = append(result, *b)
	}
	return result
}

// GetBot returns a single bot by ID.
func (m *Manager) GetBot(botID string) *BotRegistration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bots[botID]
}

// ProcessMessage routes a message to matching bots based on command prefix.
// Returns the list of bot IDs that processed the message.
func (m *Manager) ProcessMessage(msg Message) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var triggered []string

	// Check if message text starts with any registered command
	for cmd, botID := range m.commandIndex {
		if len(msg.Text) >= len(cmd) && msg.Text[:len(cmd)] == cmd {
			handler, ok := m.handlers[botID]
			if !ok {
				continue
			}
			reg := m.bots[botID]
			if reg == nil || !reg.Active {
				continue
			}
			// Run handler in a goroutine to avoid blocking
			go func(h BotHandler, m Message) {
				if err := h.OnMessage(m); err != nil {
					log.Printf("[bot] handler error: %v", err)
				}
			}(handler, msg)
			triggered = append(triggered, botID)
		}
	}

	return triggered
}

// --- Echo Bot (built-in example) ---

// EchoBot is a simple bot that echoes back messages.
type EchoBot struct{}

func (e *EchoBot) OnMessage(msg Message) error {
	log.Printf("[bot:echo] received from %s: %s", msg.From, msg.Text)
	return nil
}

func (e *EchoBot) Name() string { return "EchoBot" }

// --- Sticker Pack Types ---

// Sticker represents a single sticker in a pack.
type Sticker struct {
	ID     string `json:"id"`
	PackID string `json:"packId"`
	URL    string `json:"url"`
	Emoji  string `json:"emoji"`
}

// StickerPack represents a collection of stickers.
type StickerPack struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerNpub string    `json:"ownerNpub"`
	Stickers  []Sticker `json:"stickers"`
	CreatedAt int64     `json:"createdAt"`
}

// StickerManager manages sticker packs in memory.
type StickerManager struct {
	mu    sync.RWMutex
	packs map[string]*StickerPack // packID → pack
}

// NewStickerManager creates a new sticker manager.
func NewStickerManager() *StickerManager {
	return &StickerManager{
		packs: make(map[string]*StickerPack),
	}
}

// CreatePack creates a new sticker pack.
func (sm *StickerManager) CreatePack(name, ownerNpub string, stickers []Sticker) *StickerPack {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	id := fmt.Sprintf("spk-%d-%d", time.Now().UnixNano(), len(sm.packs))
	pack := &StickerPack{
		ID:        id,
		Name:      name,
		OwnerNpub: ownerNpub,
		Stickers:  stickers,
		CreatedAt: time.Now().Unix(),
	}
	// Assign IDs to stickers if missing
	for i := range pack.Stickers {
		if pack.Stickers[i].ID == "" {
			pack.Stickers[i].ID = fmt.Sprintf("%s-stk-%d", id, i)
		}
		if pack.Stickers[i].PackID == "" {
			pack.Stickers[i].PackID = id
		}
	}
	sm.packs[id] = pack
	return pack
}

// ListPacks returns all sticker packs (without stickers for brevity).
func (sm *StickerManager) ListPacks() []StickerPack {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]StickerPack, 0, len(sm.packs))
	for _, p := range sm.packs {
		result = append(result, StickerPack{
			ID:        p.ID,
			Name:      p.Name,
			OwnerNpub: p.OwnerNpub,
			Stickers:  p.Stickers,
			CreatedAt: p.CreatedAt,
		})
	}
	return result
}

// GetPack returns a single sticker pack by ID.
func (sm *StickerManager) GetPack(packID string) (*StickerPack, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	p, ok := sm.packs[packID]
	if !ok {
		return nil, fmt.Errorf("sticker pack %s not found", packID)
	}
	// Return a copy
	cp := *p
	return &cp, nil
}

// MarshalJSON is a helper for serializing bot data.
func MarshalBotData(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
