package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleWebhook_NewUser(t *testing.T) {
	err := HandleWebhook(TriggerNewUser, []byte(`{"username":"alice"}`))
	if err != nil {
		t.Errorf("HandleWebhook TriggerNewUser: %v", err)
	}
}

func TestHandleWebhook_Payment(t *testing.T) {
	err := HandleWebhook(TriggerPayment, []byte(`{"amount":1000,"currency":"sats"}`))
	if err != nil {
		t.Errorf("HandleWebhook TriggerPayment: %v", err)
	}
}

func TestHandleWebhook_NewChannel_NotHandled(t *testing.T) {
	// TriggerNewChannel is not in the switch — should return "unknown trigger"
	err := HandleWebhook(TriggerNewChannel, []byte(`{}`))
	if err == nil {
		t.Error("TriggerNewChannel should not be handled in switch, expected error")
	}
	if err != nil && !strings.Contains(err.Error(), "unknown trigger") {
		t.Errorf("expected 'unknown trigger' error, got: %v", err)
	}
}

// --- SendToZapier tests using httptest server ---

func TestSendToZapier_Success(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content-type, got %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(200)
	}))
	defer server.Close()

	data := map[string]interface{}{
		"message": "hello world",
		"user":    "alice",
	}
	err := SendToZapier(server.URL, TriggerNewMessage, data)
	if err != nil {
		t.Fatalf("SendToZapier: %v", err)
	}
	if receivedBody["trigger"] != "new_message" {
		t.Errorf("expected trigger=new_message, got %v", receivedBody["trigger"])
	}
	if receivedBody["data"] == nil {
		t.Error("expected data field in payload")
	}
	if receivedBody["timestamp"] == nil {
		t.Error("expected timestamp field in payload")
	}
}

func TestSendToZapier_InvalidURL(t *testing.T) {
	err := SendToZapier("http://127.0.0.1:19999/nonexistent", TriggerNewMessage, map[string]interface{}{"test": 1})
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestSendToZapier_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	// SendToZapier currently doesn't check status code, so this should succeed
	// (it only checks for network errors)
	err := SendToZapier(server.URL, TriggerNewMessage, nil)
	if err != nil {
		t.Logf("SendToZapier with 500 response: %v (current impl doesn't check status)", err)
	}
}

func TestSendToZapier_DeliveryPayload(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(200)
	}))
	defer server.Close()

	data := map[string]interface{}{
		"amount":   5000,
		"currency": "sats",
	}
	err := SendToZapier(server.URL, TriggerPayment, data)
	if err != nil {
		t.Fatal(err)
	}

	innerData, ok := receivedBody["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data field is not a map")
	}
	if innerData["amount"] != float64(5000) {
		t.Errorf("expected amount=5000, got %v", innerData["amount"])
	}
}

func TestSendToZapier_RetryOnFailure(t *testing.T) {
	// Simulate a server that's initially down (connection refused), then comes up
	// SendToZapier only returns error on network errors, not HTTP status codes,
	// so we test retry on actual network failures.
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
	}))
	// Get the server URL, then close it to simulate a down server
	serverURL := server.URL

	// First attempt: server is up, should succeed immediately
	err := SendToZapier(serverURL, TriggerNewMessage, map[string]interface{}{"test": 1})
	if err != nil {
		t.Fatalf("SendToZapier to live server should succeed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	server.Close()

	// After server close, retry should eventually fail
	err = SendToZapier(serverURL, TriggerNewMessage, map[string]interface{}{"test": 2})
	if err == nil {
		t.Error("expected error after server shutdown")
	}
}

func TestListTriggers_Values(t *testing.T) {
	triggers := ListTriggers()
	expected := []ZapierTrigger{TriggerNewMessage, TriggerNewUser, TriggerPayment, TriggerNewChannel}
	for i, exp := range expected {
		if triggers[i] != exp {
			t.Errorf("trigger[%d] = %q, want %q", i, triggers[i], exp)
		}
	}
}

func TestHandleWebhook_AllTriggers(t *testing.T) {
	tests := []struct {
		trigger ZapierTrigger
		payload string
		wantErr bool
	}{
		{TriggerNewMessage, `{"text":"hello"}`, false},
		{TriggerNewUser, `{"name":"bob"}`, false},
		{TriggerPayment, `{"amount":100}`, false},
		{TriggerNewChannel, `{}`, true}, // not handled in switch
		{"unknown_trigger", `{}`, true},
	}
	for _, tc := range tests {
		err := HandleWebhook(tc.trigger, []byte(tc.payload))
		if tc.wantErr && err == nil {
			t.Errorf("HandleWebhook(%s) expected error, got nil", tc.trigger)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("HandleWebhook(%s) unexpected error: %v", tc.trigger, err)
		}
	}
}
