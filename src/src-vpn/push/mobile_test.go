package push

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	clearProxyEnv()
	os.Exit(m.Run())
}

func clearProxyEnv() {
	for _, k := range []string{
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
		"http_proxy", "https_proxy", "all_proxy", "no_proxy",
	} {
		_ = os.Unsetenv(k)
	}
}

func TestSendFCM_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "key=test-fcm-key" {
			t.Errorf("auth header mismatch: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type mismatch")
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["to"] != "device123" {
			t.Errorf("to field mismatch: %v", body["to"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{FCMServerKey: "test-fcm-key"})
	// Override client to use test server
	p.client = srv.Client()

	// We need to override the URL. Since SendFCM hardcodes the FCM URL,
	// use a custom transport to redirect
	p.client.Transport = &redirectTransport{
		url:       srv.URL,
		transport: &http.Transport{Proxy: nil},
	}

	err := p.SendFCM("device123", PushPayload{
		Title: "Hello",
		Body:  "World",
		Sound: "default",
		Badge: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendFCM_WithData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		data, ok := body["data"].(map[string]interface{})
		if !ok {
			t.Error("data field missing")
		}
		if data["key"] != "value" {
			t.Errorf("data key mismatch: %v", data)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{FCMServerKey: "key"})
	p.client = srv.Client()
	p.client.Transport = &redirectTransport{url: srv.URL, transport: &http.Transport{Proxy: nil}}

	err := p.SendFCM("dev1", PushPayload{
		Title: "Test",
		Body:  "Body",
		Data:  map[string]string{"key": "value"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendFCM_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{FCMServerKey: "key"})
	p.client = srv.Client()
	p.client.Transport = &redirectTransport{url: srv.URL, transport: &http.Transport{Proxy: nil}}

	err := p.SendFCM("dev1", PushPayload{Title: "T", Body: "B"})
	if err == nil {
		t.Error("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestSendAPNs_Success_Sandbox(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/3/device/") {
			t.Errorf("path should contain /3/device/: %s", r.URL.Path)
		}
		if r.Header.Get("authorization") != "bearer test-apns-token" {
			t.Errorf("auth header mismatch: %s", r.Header.Get("authorization"))
		}
		if r.Header.Get("apns-topic") != "com.test.app" {
			t.Errorf("topic mismatch: %s", r.Header.Get("apns-topic"))
		}
		if r.Header.Get("apns-push-type") != "alert" {
			t.Errorf("push-type mismatch: %s", r.Header.Get("apns-push-type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{
		APNSToken:    "test-apns-token",
		APNSBundleID: "com.test.app",
	})
	p.client = srv.Client()
	p.client.Transport = &redirectTransport{url: srv.URL, transport: &http.Transport{Proxy: nil}}

	err := p.SendAPNs("device456", PushPayload{
		Title: "Alert",
		Body:  "Message",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendAPNs_Production(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{
		APNSToken:      "token",
		APNSBundleID:   "com.app",
		APNSProduction: true,
	})
	p.client = srv.Client()
	p.client.Transport = &redirectTransport{url: srv.URL, transport: &http.Transport{Proxy: nil}}

	err := p.SendAPNs("dev1", PushPayload{Title: "T", Body: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("should have called server")
	}
}

func TestSendAPNs_WithCollapseID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apns-collapse-id") != "collapse-123" {
			t.Errorf("collapse-id mismatch: %s", r.Header.Get("apns-collapse-id"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{
		APNSToken:    "token",
		APNSBundleID: "com.app",
	})
	p.client = srv.Client()
	p.client.Transport = &redirectTransport{url: srv.URL, transport: &http.Transport{Proxy: nil}}

	err := p.SendAPNs("dev1", PushPayload{
		Title:      "T",
		Body:       "B",
		CollapseID: "collapse-123",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendAPNs_WithData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		data, ok := body["data"].(map[string]interface{})
		if !ok || data["custom"] != "val" {
			t.Errorf("data missing or wrong: %v", body["data"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{
		APNSToken:    "token",
		APNSBundleID: "com.app",
	})
	p.client = srv.Client()
	p.client.Transport = &redirectTransport{url: srv.URL, transport: &http.Transport{Proxy: nil}}

	err := p.SendAPNs("dev1", PushPayload{
		Title: "T",
		Body:  "B",
		Data:  map[string]string{"custom": "val"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendAPNs_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{
		APNSToken:    "token",
		APNSBundleID: "com.app",
	})
	p.client = srv.Client()
	p.client.Transport = &redirectTransport{url: srv.URL, transport: &http.Transport{Proxy: nil}}

	err := p.SendAPNs("dev1", PushPayload{Title: "T", Body: "B"})
	if err == nil {
		t.Error("expected error for 410")
	}
	if !strings.Contains(err.Error(), "410") {
		t.Errorf("error should mention status: %v", err)
	}
}

func TestSend_PrefersFCM(t *testing.T) {
	fcmCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fcmCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{
		FCMServerKey: "fcm-key",
		APNSToken:    "apns-token",
		APNSBundleID: "com.app",
	})
	p.client = srv.Client()
	p.client.Transport = &redirectTransport{url: srv.URL, transport: &http.Transport{Proxy: nil}}

	err := p.Send("dev1", PushPayload{Title: "T", Body: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if !fcmCalled {
		t.Error("should have used FCM")
	}
}

func TestSend_FallbackToAPNs(t *testing.T) {
	apnsCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apnsCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewMobilePusher(MobilePushConfig{
		APNSToken:    "apns-token",
		APNSBundleID: "com.app",
	})
	p.client = srv.Client()
	p.client.Transport = &redirectTransport{url: srv.URL, transport: &http.Transport{Proxy: nil}}

	err := p.Send("dev1", PushPayload{Title: "T", Body: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if !apnsCalled {
		t.Error("should have fallen back to APNs")
	}
}

func TestPushPayload_JSON(t *testing.T) {
	p := PushPayload{
		Title:      "Title",
		Body:       "Body",
		Sound:      "default",
		Badge:      3,
		CollapseID: "col1",
		Data:       map[string]string{"k": "v"},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var p2 PushPayload
	if err := json.Unmarshal(data, &p2); err != nil {
		t.Fatal(err)
	}
	if p2.Title != "Title" || p2.Body != "Body" || p2.Badge != 3 {
		t.Errorf("roundtrip failed: %+v", p2)
	}
}

// redirectTransport redirects all requests to the test server URL
// Base transport must not be http.DefaultTransport (honors HTTP(S)_PROXY).
// Use &http.Transport{Proxy: nil} or httptest Client.Transport.
type redirectTransport struct {
	url       string
	transport http.RoundTripper
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to test server while preserving the path
	newURL := rt.url + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}
	newReq, err := http.NewRequest(req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header
	return rt.transport.RoundTrip(newReq)
}
