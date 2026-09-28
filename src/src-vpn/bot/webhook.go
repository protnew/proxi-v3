package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// WebhookConfig defines a webhook for external integrations.
type WebhookConfig struct {
	ID     string   `json:"id"`
	BotID  string   `json:"bot_id"`
	URL    string   `json:"url"`
	Secret string   `json:"secret"`
	Events []string `json:"events"`
}

// RegisterWebhook saves a webhook configuration.
func RegisterWebhook(db *store.Store, config WebhookConfig) error {
	d := db.DB()
	events, _ := json.Marshal(config.Events)
	_, err := d.Exec(
		"INSERT OR REPLACE INTO webhooks (id, bot_id, url, secret, events) VALUES (?, ?, ?, ?, ?)",
		config.ID, config.BotID, config.URL, config.Secret, string(events),
	)
	return err
}

// ListWebhooks returns all webhooks for a bot.
func ListWebhooks(db *store.Store, botID string) ([]WebhookConfig, error) {
	d := db.DB()
	rows, err := d.Query("SELECT id, bot_id, url, secret, events FROM webhooks WHERE bot_id = ?", botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hooks []WebhookConfig
	for rows.Next() {
		var h WebhookConfig
		var eventsStr string
		rows.Scan(&h.ID, &h.BotID, &h.URL, &h.Secret, &eventsStr)
		json.Unmarshal([]byte(eventsStr), &h.Events)
		hooks = append(hooks, h)
	}
	return hooks, nil
}

// TriggerWebhook sends an event payload to the webhook URL.
func TriggerWebhook(config WebhookConfig, event string, payload []byte) error {
	body, _ := json.Marshal(map[string]interface{}{
		"event":     event,
		"bot_id":    config.BotID,
		"payload":   json.RawMessage(payload),
		"timestamp": time.Now().Unix(),
	})
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(config.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook %s: %w", config.URL, err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}
