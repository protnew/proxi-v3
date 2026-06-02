package tor

import (
	"testing"
)

func TestNewTorDialer(t *testing.T) {
	t.Parallel()

	t.Run("empty address uses default", func(t *testing.T) {
		t.Parallel()
		d := NewTorDialer("")
		if d.proxyAddr != "127.0.0.1:9050" {
			t.Errorf("expected default proxy 127.0.0.1:9050, got %s", d.proxyAddr)
		}
	})

	t.Run("custom address is preserved", func(t *testing.T) {
		t.Parallel()
		d := NewTorDialer("192.168.1.1:9051")
		if d.proxyAddr != "192.168.1.1:9051" {
			t.Errorf("expected proxy 192.168.1.1:9051, got %s", d.proxyAddr)
		}
	})
}

func TestIsTorRunning(t *testing.T) {
	t.Parallel()

	d := NewTorDialer("127.0.0.1:9050")
	running := d.IsTorRunning()
	// In a CI/test environment Tor is very unlikely to be running.
	// We just verify the call completes without panicking.
	t.Logf("IsTorRunning = %v (expected false in test env)", running)
	if running {
		t.Log("Tor appears to be running on this machine")
	}
}

func TestGetOnionAddress_Fallback(t *testing.T) {
	t.Parallel()

	// Use a non-existent data dir so no hostname file is found,
	// and the control port is unavailable → should return fallback.
	addr := GetOnionAddress("/tmp/nonexistent-tor-test-dir-12345")
	if addr != "not-yet-configured.onion" {
		t.Errorf("expected fallback onion address, got %q", addr)
	}
}

func TestHTTPTransport(t *testing.T) {
	t.Parallel()

	d := NewTorDialer("127.0.0.1:9050")
	tr := d.HTTPTransport()
	if tr == nil {
		t.Fatal("HTTPTransport returned nil")
	}
	// HTTPTransport returns *http.Transport with DialContext set.
	if tr.DialContext == nil {
		t.Error("Transport.DialContext is nil")
	}
}
