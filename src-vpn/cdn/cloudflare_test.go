package cdn

import "testing"

func TestGetURL(t *testing.T) {
	m := NewCDNManager("test-token", "test-zone")

	url := m.GetURL("avatars/user1.png")
	expected := "https://cdn.unkillable-messenger.com/avatars/user1.png"
	if url != expected {
		t.Errorf("GetURL = %q, want %q", url, expected)
	}

	url2 := m.GetURL("files/doc.pdf")
	expected2 := "https://cdn.unkillable-messenger.com/files/doc.pdf"
	if url2 != expected2 {
		t.Errorf("GetURL = %q, want %q", url2, expected2)
	}
}

func TestPurgeCacheNoToken(t *testing.T) {
	m := NewCDNManager("", "test-zone")
	err := m.PurgeCache([]string{"https://example.com/file.js"})
	if err == nil {
		t.Error("expected error with empty token")
	}
}

func TestUploadFileNoToken(t *testing.T) {
	m := NewCDNManager("", "test-zone")
	_, err := m.UploadFile("test.txt", []byte("hello"))
	if err == nil {
		t.Error("expected error with empty token")
	}
}

func TestNewCDNManager(t *testing.T) {
	m := NewCDNManager("tok123", "zone456")
	if m.apiToken != "tok123" {
		t.Errorf("apiToken = %q, want %q", m.apiToken, "tok123")
	}
	if m.zoneID != "zone456" {
		t.Errorf("zoneID = %q, want %q", m.zoneID, "zone456")
	}
	if m.baseURL != "https://api.cloudflare.com/client/v4/zones/zone456" {
		t.Errorf("baseURL = %q", m.baseURL)
	}
}
