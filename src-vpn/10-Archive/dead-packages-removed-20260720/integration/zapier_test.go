package integration

import "testing"

func TestListTriggers(t *testing.T) {
	triggers := ListTriggers()
	if len(triggers) != 4 {
		t.Errorf("expected 4 triggers, got %d", len(triggers))
	}
}

func TestHandleWebhook(t *testing.T) {
	err := HandleWebhook(TriggerNewMessage, []byte(`{"text":"hello"}`))
	if err != nil {
		t.Error(err)
	}
	err = HandleWebhook("unknown", []byte(`{}`))
	if err == nil {
		t.Error("unknown trigger should fail")
	}
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	err := HandleWebhook(TriggerNewMessage, []byte(`not json`))
	if err == nil {
		t.Error("invalid JSON should fail")
	}
}
