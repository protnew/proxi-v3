package bot

import (
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

func newTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRegisterAndListWebhooks(t *testing.T) {
	db := newTestDB(t)
	config := WebhookConfig{
		ID:     "wh1",
		BotID:  "bot1",
		URL:    "https://example.com/webhook",
		Secret: "secret123",
		Events: []string{"message", "payment"},
	}
	if err := RegisterWebhook(db, config); err != nil {
		t.Fatal(err)
	}

	hooks, err := ListWebhooks(db, "bot1")
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(hooks))
	}
	if hooks[0].URL != "https://example.com/webhook" {
		t.Errorf("wrong URL: %s", hooks[0].URL)
	}
	if len(hooks[0].Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(hooks[0].Events))
	}
}

func TestTriggerWebhook_Fails(t *testing.T) {
	config := WebhookConfig{BotID: "bot1", URL: "http://localhost:19999/webhook"}
	err := TriggerWebhook(config, "test", []byte(`{"hello":"world"}`))
	if err == nil {
		t.Error("should fail: no server on 19999")
	}
}
