package bot

import (
	"sync"
	"testing"
	"time"
)

// testBot is a simple test bot that records messages.
type testBot struct {
	mu   sync.Mutex
	name string
	msgs []Message
	err  error
}

func (b *testBot) OnMessage(msg Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, msg)
	return b.err
}

func (b *testBot) Name() string { return b.name }

func (b *testBot) getMessages() []Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.msgs
}

// --- Bot Manager Tests ---

func TestManager_RegisterBot(t *testing.T) {
	t.Parallel()

	m := NewManager()
	bot := &testBot{name: "TestBot"}
	reg := m.RegisterBot("TestBot", "user1", "/test", bot)

	if reg.ID == "" {
		t.Fatal("bot ID should not be empty")
	}
	if reg.Name != "TestBot" {
		t.Fatalf("expected name TestBot, got %s", reg.Name)
	}
	if reg.Command != "/test" {
		t.Fatalf("expected command /test, got %s", reg.Command)
	}
	if !reg.Active {
		t.Fatal("bot should be active")
	}
}

func TestManager_ListBots(t *testing.T) {
	t.Parallel()

	m := NewManager()
	m.RegisterBot("Bot1", "user1", "/bot1", &testBot{name: "Bot1"})
	m.RegisterBot("Bot2", "user2", "/bot2", &testBot{name: "Bot2"})

	bots := m.ListBots()
	if len(bots) != 2 {
		t.Fatalf("expected 2 bots, got %d", len(bots))
	}
}

func TestManager_UnregisterBot(t *testing.T) {
	t.Parallel()

	m := NewManager()
	reg := m.RegisterBot("Bot1", "user1", "/bot1", &testBot{name: "Bot1"})

	if err := m.UnregisterBot(reg.ID); err != nil {
		t.Fatalf("UnregisterBot: %v", err)
	}

	bots := m.ListBots()
	if len(bots) != 0 {
		t.Fatalf("expected 0 bots after unregister, got %d", len(bots))
	}
}

func TestManager_UnregisterBot_NotFound(t *testing.T) {
	t.Parallel()

	m := NewManager()
	err := m.UnregisterBot("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent bot")
	}
}

func TestManager_GetBot(t *testing.T) {
	t.Parallel()

	m := NewManager()
	reg := m.RegisterBot("Bot1", "user1", "/bot1", &testBot{name: "Bot1"})

	got := m.GetBot(reg.ID)
	if got == nil {
		t.Fatal("expected bot, got nil")
	}
	if got.Name != "Bot1" {
		t.Fatalf("expected name Bot1, got %s", got.Name)
	}

	// Non-existent
	if m.GetBot("nope") != nil {
		t.Fatal("expected nil for non-existent bot")
	}
}

func TestManager_ProcessMessage(t *testing.T) {
	t.Parallel()

	m := NewManager()
	bot1 := &testBot{name: "EchoBot"}
	m.RegisterBot("EchoBot", "user1", "/echo", bot1)

	msg := Message{
		ID:        "msg-1",
		From:      "alice",
		To:        "broadcast",
		Text:      "/echo hello world",
		Timestamp: time.Now().Unix(),
		Type:      "chat",
	}

	triggered := m.ProcessMessage(msg)
	if len(triggered) != 1 {
		t.Fatalf("expected 1 triggered bot, got %d", len(triggered))
	}

	// Wait for async handler
	time.Sleep(50 * time.Millisecond)

	msgs := bot1.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message received by bot, got %d", len(msgs))
	}
	if msgs[0].Text != "/echo hello world" {
		t.Fatalf("unexpected message text: %s", msgs[0].Text)
	}
}

func TestManager_ProcessMessage_NoMatch(t *testing.T) {
	t.Parallel()

	m := NewManager()
	bot1 := &testBot{name: "EchoBot"}
	m.RegisterBot("EchoBot", "user1", "/echo", bot1)

	msg := Message{
		Text: "just a regular message",
	}

	triggered := m.ProcessMessage(msg)
	if len(triggered) != 0 {
		t.Fatalf("expected 0 triggered bots, got %d", len(triggered))
	}
}

func TestManager_ProcessMessage_MultipleBots(t *testing.T) {
	t.Parallel()

	m := NewManager()
	bot1 := &testBot{name: "EchoBot"}
	bot2 := &testBot{name: "GreetBot"}
	m.RegisterBot("EchoBot", "u1", "/echo", bot1)
	m.RegisterBot("GreetBot", "u2", "/greet", bot2)

	msg := Message{Text: "/echo test", From: "alice"}
	triggered := m.ProcessMessage(msg)

	if len(triggered) != 1 {
		t.Fatalf("expected 1 triggered (echo only), got %d", len(triggered))
	}
	if triggered[0] != "EchoBot" && len(triggered) > 0 {
		// commandIndex maps command → botID, check the bot is echo
		b := m.GetBot(triggered[0])
		if b.Name != "EchoBot" {
			t.Fatalf("expected EchoBot, got %s", b.Name)
		}
	}
}

func TestEchoBot(t *testing.T) {
	t.Parallel()

	echo := &EchoBot{}
	if echo.Name() != "EchoBot" {
		t.Fatalf("expected EchoBot, got %s", echo.Name())
	}

	err := echo.OnMessage(Message{Text: "hello"})
	if err != nil {
		t.Fatalf("EchoBot.OnMessage: %v", err)
	}
}

// --- Sticker Manager Tests ---

func TestStickerManager_CreatePack(t *testing.T) {
	t.Parallel()

	sm := NewStickerManager()
	stickers := []Sticker{
		{URL: "https://example.com/s1.webp", Emoji: "😀"},
		{URL: "https://example.com/s2.webp", Emoji: "🎉"},
	}

	pack := sm.CreatePack("Fun Pack", "user1", stickers)
	if pack.ID == "" {
		t.Fatal("pack ID should not be empty")
	}
	if pack.Name != "Fun Pack" {
		t.Fatalf("expected name 'Fun Pack', got %s", pack.Name)
	}
	if len(pack.Stickers) != 2 {
		t.Fatalf("expected 2 stickers, got %d", len(pack.Stickers))
	}

	// Check sticker IDs were assigned
	for i, s := range pack.Stickers {
		if s.ID == "" {
			t.Fatalf("sticker %d should have an ID", i)
		}
		if s.PackID != pack.ID {
			t.Fatalf("sticker %d should have packID %s, got %s", i, pack.ID, s.PackID)
		}
	}
}

func TestStickerManager_ListPacks(t *testing.T) {
	t.Parallel()

	sm := NewStickerManager()
	sm.CreatePack("Pack1", "u1", nil)
	sm.CreatePack("Pack2", "u2", nil)

	packs := sm.ListPacks()
	if len(packs) != 2 {
		t.Fatalf("expected 2 packs, got %d", len(packs))
	}
}

func TestStickerManager_GetPack(t *testing.T) {
	t.Parallel()

	sm := NewStickerManager()
	pack := sm.CreatePack("TestPack", "u1", []Sticker{
		{URL: "https://example.com/s1.webp", Emoji: "😀"},
	})

	got, err := sm.GetPack(pack.ID)
	if err != nil {
		t.Fatalf("GetPack: %v", err)
	}
	if got.Name != "TestPack" {
		t.Fatalf("expected TestPack, got %s", got.Name)
	}
	if len(got.Stickers) != 1 {
		t.Fatalf("expected 1 sticker, got %d", len(got.Stickers))
	}
}

func TestStickerManager_GetPack_NotFound(t *testing.T) {
	t.Parallel()

	sm := NewStickerManager()
	_, err := sm.GetPack("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent pack")
	}
}

func TestMarshalBotData(t *testing.T) {
	t.Parallel()

	data, err := MarshalBotData(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("MarshalBotData: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty data")
	}
}
