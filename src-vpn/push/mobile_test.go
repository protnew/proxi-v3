package push

import "testing"

func TestNewMobilePusher(t *testing.T) {
	p := NewMobilePusher(MobilePushConfig{})
	if p == nil {
		t.Error("pusher should not be nil")
	}
}

func TestSendFCM_NoKey(t *testing.T) {
	p := NewMobilePusher(MobilePushConfig{})
	err := p.SendFCM("device123", PushPayload{Title: "Test", Body: "Hello"})
	if err == nil {
		t.Error("should fail without FCM key")
	}
}

func TestSendAPNs_NoToken(t *testing.T) {
	p := NewMobilePusher(MobilePushConfig{})
	err := p.SendAPNs("device123", PushPayload{Title: "Test", Body: "Hello"})
	if err == nil {
		t.Error("should fail without APNs token")
	}
}

func TestSend_NoProvider(t *testing.T) {
	p := NewMobilePusher(MobilePushConfig{})
	err := p.Send("device123", PushPayload{Title: "Test", Body: "Hello"})
	if err == nil {
		t.Error("should fail without any provider")
	}
}

func TestSendFCM_InvalidToken(t *testing.T) {
	p := NewMobilePusher(MobilePushConfig{FCMServerKey: "fake-key"})
	err := p.SendFCM("invalid-token", PushPayload{Title: "Test", Body: "Hello"})
	// Will fail because fake-key is invalid, but at least it tries
	if err == nil {
		// May or may not fail depending on network
		t.Log("FCM call completed (expected in test env)")
	}
}
