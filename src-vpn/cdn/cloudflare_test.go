package cdn

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	if m.r2URL != "https://zone456.r2.cloudflarestorage.com" {
		t.Errorf("r2URL = %q", m.r2URL)
	}
	if m.client == nil {
		t.Error("client is nil")
	}
}

func TestPurgeCache_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/purge_cache" {
			t.Errorf("expected /purge_cache, got %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m := NewCDNManager("test-token", "test-zone")
	m.baseURL = server.URL
	m.client = server.Client()

	err := m.PurgeCache([]string{"https://example.com/file.js"})
	if err != nil {
		t.Fatalf("PurgeCache failed: %v", err)
	}
}

func TestPurgeCache_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "internal error")
	}))
	defer server.Close()

	m := NewCDNManager("test-token", "test-zone")
	m.baseURL = server.URL
	m.client = server.Client()

	err := m.PurgeCache([]string{"https://example.com/file.js"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !contains(err.Error(), "status 500") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUploadFile_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("Authorization = %q", auth)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("Content-Type = %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "hello world" {
			t.Errorf("body = %q, want %q", string(body), "hello world")
		}
		if r.URL.Path != "/test/file.txt" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/test/file.txt")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m := NewCDNManager("test-token", "test-zone")
	m.r2URL = server.URL
	m.client = server.Client()

	url, err := m.UploadFile("test/file.txt", []byte("hello world"))
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}
	expected := "https://cdn.unkillable-messenger.com/test/file.txt"
	if url != expected {
		t.Errorf("url = %q, want %q", url, expected)
	}
}

func TestUploadFile_Created(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	m := NewCDNManager("test-token", "test-zone")
	m.r2URL = server.URL
	m.client = server.Client()

	url, err := m.UploadFile("new.txt", []byte("data"))
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}
	if url != "https://cdn.unkillable-messenger.com/new.txt" {
		t.Errorf("url = %q", url)
	}
}

func TestUploadFile_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "access denied")
	}))
	defer server.Close()

	m := NewCDNManager("test-token", "test-zone")
	m.r2URL = server.URL
	m.client = server.Client()

	_, err := m.UploadFile("denied.txt", []byte("data"))
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !contains(err.Error(), "status 403") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUploadFile_VariousSizes(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"empty", 0},
		{"small", 100},
		{"medium", 1024},
		{"large", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.size)
			for i := range data {
				data[i] = byte(i % 256)
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if len(body) != tt.size {
					t.Errorf("body size = %d, want %d", len(body), tt.size)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			m := NewCDNManager("test-token", "test-zone")
			m.r2URL = server.URL
			m.client = server.Client()

			_, err := m.UploadFile(fmt.Sprintf("file_%s.bin", tt.name), data)
			if err != nil {
				t.Fatalf("UploadFile failed: %v", err)
			}
		})
	}
}

func TestGetURL_EmptyKey(t *testing.T) {
	m := NewCDNManager("token", "zone")
	url := m.GetURL("")
	expected := "https://cdn.unkillable-messenger.com/"
	if url != expected {
		t.Errorf("GetURL('') = %q, want %q", url, expected)
	}
}

func TestGetURL_NestedPath(t *testing.T) {
	m := NewCDNManager("token", "zone")
	url := m.GetURL("a/b/c/d/file.dat")
	expected := "https://cdn.unkillable-messenger.com/a/b/c/d/file.dat"
	if url != expected {
		t.Errorf("GetURL = %q, want %q", url, expected)
	}
}

func TestPurgeCache_MultipleURLs(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m := NewCDNManager("test-token", "test-zone")
	m.baseURL = server.URL
	m.client = server.Client()

	urls := []string{"https://a.com/1", "https://b.com/2", "https://c.com/3"}
	err := m.PurgeCache(urls)
	if err != nil {
		t.Fatalf("PurgeCache failed: %v", err)
	}
	if receivedBody == "" {
		t.Error("no body received")
	}
	t.Logf("PurgeCache body: %s", receivedBody)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
