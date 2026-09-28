package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MobilePushConfig holds APNs and FCM credentials.
type MobilePushConfig struct {
	// APNs (Apple)
	APNSToken      string // p8 token key
	APNSTeamID     string
	APNSKeyID      string
	APNSBundleID   string
	APNSProduction bool

	// FCM (Firebase Cloud Messaging)
	FCMServerKey string
	FCMProjectID string
}

// MobilePusher sends push notifications via APNs and FCM.
type MobilePusher struct {
	config MobilePushConfig
	client *http.Client
}

// NewMobilePusher creates a new mobile push sender.
func NewMobilePusher(config MobilePushConfig) *MobilePusher {
	return &MobilePusher{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// PushPayload is a universal push notification payload.
type PushPayload struct {
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Sound      string            `json:"sound,omitempty"` // "default" or custom
	Badge      int               `json:"badge,omitempty"`
	CollapseID string            `json:"collapse_id,omitempty"`
	Data       map[string]string `json:"data,omitempty"`
}

// SendFCM sends a push notification via Firebase Cloud Messaging (Android/iOS).
func (p *MobilePusher) SendFCM(deviceToken string, payload PushPayload) error {
	if p.config.FCMServerKey == "" {
		return fmt.Errorf("FCM server key not configured")
	}

	body := map[string]interface{}{
		"to": deviceToken,
		"notification": map[string]string{
			"title": payload.Title,
			"body":  payload.Body,
		},
		"android": map[string]interface{}{
			"priority": "high",
		},
		"apns": map[string]interface{}{
			"payload": map[string]interface{}{
				"aps": map[string]interface{}{
					"sound": payload.Sound,
					"badge": payload.Badge,
				},
			},
		},
	}
	if len(payload.Data) > 0 {
		body["data"] = payload.Data
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "key="+p.config.FCMServerKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("FCM request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("FCM returned %d", resp.StatusCode)
	}
	return nil
}

// SendAPNs sends a push notification via Apple Push Notification service.
func (p *MobilePusher) SendAPNs(deviceToken string, payload PushPayload) error {
	if p.config.APNSToken == "" {
		return fmt.Errorf("APNs token not configured")
	}

	host := "https://api.sandbox.push.apple.com"
	if p.config.APNSProduction {
		host = "https://api.push.apple.com"
	}
	url := fmt.Sprintf("%s/3/device/%s", host, deviceToken)

	apnsPayload := map[string]interface{}{
		"aps": map[string]interface{}{
			"alert": map[string]string{
				"title": payload.Title,
				"body":  payload.Body,
			},
			"sound": payload.Sound,
			"badge": payload.Badge,
		},
	}
	if len(payload.Data) > 0 {
		apnsPayload["data"] = payload.Data
	}

	jsonBody, _ := json.Marshal(apnsPayload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	req.Header.Set("authorization", "bearer "+p.config.APNSToken)
	req.Header.Set("apns-topic", p.config.APNSBundleID)
	req.Header.Set("apns-push-type", "alert")
	if payload.CollapseID != "" {
		req.Header.Set("apns-collapse-id", payload.CollapseID)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("APNs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("APNs returned %d", resp.StatusCode)
	}
	return nil
}

// Send sends push to the best available provider.
func (p *MobilePusher) Send(deviceToken string, payload PushPayload) error {
	// Try FCM first (works for both Android and iOS via Firebase)
	if p.config.FCMServerKey != "" {
		return p.SendFCM(deviceToken, payload)
	}
	// Fall back to APNs
	if p.config.APNSToken != "" {
		return p.SendAPNs(deviceToken, payload)
	}
	return fmt.Errorf("no push provider configured")
}
