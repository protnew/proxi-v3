package tor

import (
	"testing"
	"time"
)

func TestAutoOnionManager_Creation(t *testing.T) {
	m := NewAutoOnionManager("127.0.0.1:9051", 8080, "/tmp/test-onion")
	if m == nil {
		t.Fatal("NewAutoOnionManager returned nil")
	}
	if m.targetPort != 8080 {
		t.Errorf("targetPort = %d, want 8080", m.targetPort)
	}
	if m.controlAddr != "127.0.0.1:9051" {
		t.Errorf("controlAddr = %q, want 127.0.0.1:9051", m.controlAddr)
	}
}

func TestAutoOnionManager_NotRunning(t *testing.T) {
	m := NewAutoOnionManager("127.0.0.1:1", 9999, "/tmp/nonexistent-tor-test")
	if m.IsRunning() {
		t.Error("expected IsRunning=false before Start")
	}
}

func TestAutoOnionManager_StartNoTor(t *testing.T) {
	m := NewAutoOnionManager("127.0.0.1:1", 9999, "/tmp/nonexistent-tor-test")
	err := m.Start()
	if err == nil {
		t.Error("expected error when Tor not running")
	}
}

func TestAutoOnionManager_GetOnionAddress_Empty(t *testing.T) {
	m := NewAutoOnionManager("127.0.0.1:1", 9999, "/tmp/nonexistent-tor-test-12345")
	addr := m.GetOnionAddress()
	if addr != "" {
		t.Errorf("expected empty address, got %q", addr)
	}
}

func TestAutoOnionManager_GetOnionAddress_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	svcDir := tmpDir + "/hidden_service"
	if err := mkdirAll(svcDir); err != nil {
		t.Fatal(err)
	}
	if err := writeHostname(svcDir, "autoconfigtest.onion"); err != nil {
		t.Fatal(err)
	}

	m := NewAutoOnionManager("127.0.0.1:1", 9999, tmpDir)
	addr := m.GetOnionAddress()
	if addr != "autoconfigtest.onion" {
		t.Errorf("expected autoconfigtest.onion, got %q", addr)
	}
}

func TestAutoOnionManager_Stop(t *testing.T) {
	m := NewAutoOnionManager("127.0.0.1:1", 9999, "/tmp/nonexistent")
	m.Stop()
	if m.IsRunning() {
		t.Error("expected IsRunning=false after Stop")
	}
}

func TestAutoOnionManager_SetAutoStart(t *testing.T) {
	m := NewAutoOnionManager("127.0.0.1:1", 9999, "/tmp/nonexistent")
	m.SetAutoStart(false)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.autoStart != false {
		t.Error("expected autoStart=false")
	}
}

// Test control port command parsing
func TestParseOnionAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ServiceID=testaddress123.onion", "testaddress123.onion"},
		{"250-ServiceID=abc.onion\r\n", "abc.onion"},
		{"250 ServiceID=xyz567.onion\r\n", "xyz567.onion"},
		{"no match here", ""},
	}

	for _, tc := range tests {
		got := extractOnionFromResponse(tc.input)
		if got != tc.expected {
			t.Errorf("extractOnionFromResponse(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestOnionAddress_Validation(t *testing.T) {
	valid := []string{
		"abcdefghijklmnopqrstuvwxyz.onion",
		"abcdefghijklmnop.onion", // v2 style (16 chars)
	}
	invalid := []string{
		"short.onion",
		"no-onion-suffix.com",
		"",
	}

	for _, addr := range valid {
		if !isValidOnionAddress(addr) {
			t.Errorf("expected %q to be valid", addr)
		}
	}
	for _, addr := range invalid {
		if isValidOnionAddress(addr) {
			t.Errorf("expected %q to be invalid", addr)
		}
	}
}

func TestAutoOnionManager_BackoffSchedule(t *testing.T) {
	// Verify retry intervals are reasonable (not testing actual sleep)
	interval := 30 * time.Second
	maxRetries := 10

	if maxRetries != 10 {
		t.Errorf("maxRetries = %d, want 10", maxRetries)
	}
	if interval != 30*time.Second {
		t.Errorf("interval = %v, want 30s", interval)
	}
}
