package monitoring

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIncrementCounter(t *testing.T) {
	m := NewMetrics()
	m.IncrementCounter("messages_sent")
	m.IncrementCounter("messages_sent")
	m.IncrementCounter("errors")

	if m.counters["messages_sent"] != 2 {
		t.Errorf("messages_sent = %g, want 2", m.counters["messages_sent"])
	}
	if m.counters["errors"] != 1 {
		t.Errorf("errors = %g, want 1", m.counters["errors"])
	}
}

func TestSetGauge(t *testing.T) {
	m := NewMetrics()
	m.SetGauge("active_users", 42.0)
	m.SetGauge("active_users", 50.0)

	if m.gauges["active_users"] != 50.0 {
		t.Errorf("active_users = %g, want 50", m.gauges["active_users"])
	}
}

func TestObserveLatency(t *testing.T) {
	m := NewMetrics()
	m.ObserveLatency("api_request", 100*time.Millisecond)
	m.ObserveLatency("api_request", 200*time.Millisecond)

	if len(m.latency["api_request"]) != 2 {
		t.Errorf("latency count = %d, want 2", len(m.latency["api_request"]))
	}
}

func TestHandler(t *testing.T) {
	m := NewMetrics()
	m.IncrementCounter("messages_sent")
	m.SetGauge("active_users", 42.0)
	m.ObserveLatency("api_request", 150*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	handler := m.Handler()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body := w.Body.String()

	if !strings.Contains(body, "messages_sent_total 1") {
		t.Errorf("expected counter in body, got:\n%s", body)
	}
	if !strings.Contains(body, "active_users 42") {
		t.Errorf("expected gauge in body, got:\n%s", body)
	}
	if !strings.Contains(body, "api_request_duration_seconds_count 1") {
		t.Errorf("expected latency in body, got:\n%s", body)
	}
}

func TestHandlerEmpty(t *testing.T) {
	m := NewMetrics()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	m.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Result().StatusCode)
	}
}
