package ipfs

import (
	"testing"
)

func TestNewClient_Defaults(t *testing.T) {
	t.Parallel()

	c := NewClient("", "")
	if c.gateway != "https://ipfs.io/ipfs/" {
		t.Errorf("expected default gateway, got %q", c.gateway)
	}
	if c.apiURL != "http://127.0.0.1:5001" {
		t.Errorf("expected default apiURL, got %q", c.apiURL)
	}
	if c.httpClient == nil {
		t.Error("httpClient is nil")
	}
}

func TestIsAvailable(t *testing.T) {
	t.Parallel()

	// Point at a port where no IPFS daemon is running.
	c := NewClient("", "http://127.0.0.1:19999")
	avail := c.IsAvailable()
	if avail {
		t.Error("IsAvailable = true, expected false (no daemon on :19999)")
	}
}

func TestUploadFile_NoDaemon(t *testing.T) {
	t.Parallel()

	c := NewClient("", "http://127.0.0.1:19999")
	_, err := c.UploadFile("test.txt", []byte("hello"))
	if err == nil {
		t.Fatal("UploadFile should return error when no daemon is running")
	}
}

func TestParseInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int64
	}{
		{"12345", 12345},
		{"0", 0},
		{"999999999", 999999999},
		{"", 0},
		{"abc", 0},
	}

	for _, tc := range tests {
		got := parseInt64(tc.input)
		if got != tc.want {
			t.Errorf("parseInt64(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
