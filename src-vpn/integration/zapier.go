package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ZapierTrigger defines available trigger events.
type ZapierTrigger string

const (
	TriggerNewMessage  ZapierTrigger = "new_message"
	TriggerNewUser     ZapierTrigger = "new_user"
	TriggerPayment     ZapierTrigger = "payment"
	TriggerNewChannel  ZapierTrigger = "new_channel"
)

// HandleWebhook processes an incoming Zapier webhook.
func HandleWebhook(trigger ZapierTrigger, payload []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	// Process based on trigger type
	switch trigger {
	case TriggerNewMessage:
		return handleNewMessage(data)
	case TriggerNewUser:
		return handleNewUser(data)
	case TriggerPayment:
		return handlePayment(data)
	default:
		return fmt.Errorf("unknown trigger: %s", trigger)
	}
}

func handleNewMessage(data map[string]interface{}) error { return nil }
func handleNewUser(data map[string]interface{}) error    { return nil }
func handlePayment(data map[string]interface{}) error    { return nil }

// ListTriggers returns all available triggers.
func ListTriggers() []ZapierTrigger {
	return []ZapierTrigger{TriggerNewMessage, TriggerNewUser, TriggerPayment, TriggerNewChannel}
}

// SendToZapier pushes an event to a Zapier webhook URL.
func SendToZapier(webhookURL string, trigger ZapierTrigger, data map[string]interface{}) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"trigger":    trigger,
		"data":       data,
		"timestamp":  time.Now().Unix(),
	})
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
