package storage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewFilecoinPinner(t *testing.T) {
	fp := NewFilecoinPinner("http://localhost:5001", "test-token")
	if fp == nil {
		t.Fatal("NewFilecoinPinner returned nil")
	}
	if fp.apiURL != "http://localhost:5001" {
		t.Fatalf("expected apiURL http://localhost:5001, got %s", fp.apiURL)
	}
	if fp.token != "test-token" {
		t.Fatalf("expected token test-token, got %s", fp.token)
	}
}

func TestPinCID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pin/QmTestCID123" {
			t.Errorf("expected path /api/pin/QmTestCID123, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pinResponse{
			RequestID: "req-001",
			Status:    "pinning",
			CID:       "QmTestCID123",
		})
	}))
	defer server.Close()

	fp := NewFilecoinPinner(server.URL, "test-token")
	reqID, err := fp.PinCID("QmTestCID123")
	if err != nil {
		t.Fatalf("PinCID failed: %v", err)
	}
	if reqID != "req-001" {
		t.Fatalf("expected request ID req-001, got %s", reqID)
	}
}

func TestPinCIDError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	fp := NewFilecoinPinner(server.URL, "bad-token")
	_, err := fp.PinCID("QmTestCID123")
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}
}

func TestUnpinCID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pin/QmTestCID123" {
			t.Errorf("expected path /api/pin/QmTestCID123, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	fp := NewFilecoinPinner(server.URL, "test-token")
	if err := fp.UnpinCID("QmTestCID123"); err != nil {
		t.Fatalf("UnpinCID failed: %v", err)
	}
}

func TestUnpinCIDError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	fp := NewFilecoinPinner(server.URL, "test-token")
	err := fp.UnpinCID("QmNonExistent")
	if err == nil {
		t.Fatal("expected error for non-existent CID")
	}
}

func TestGetStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/pin/QmTestCID123/status"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statusResponse{
			Status: "pinned",
			CID:    "QmTestCID123",
		})
	}))
	defer server.Close()

	fp := NewFilecoinPinner(server.URL, "test-token")
	status, err := fp.GetStatus("QmTestCID123")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status != "pinned" {
		t.Fatalf("expected status 'pinned', got '%s'", status)
	}
}

func TestGetStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	fp := NewFilecoinPinner(server.URL, "test-token")
	_, err := fp.GetStatus("QmTestCID123")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestIsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	fp := NewFilecoinPinner(server.URL, "test-token")
	if !fp.IsAvailable() {
		t.Fatal("expected IsAvailable to return true")
	}
}

func TestIsAvailableEmptyURL(t *testing.T) {
	fp := NewFilecoinPinner("", "test-token")
	if fp.IsAvailable() {
		t.Fatal("expected IsAvailable to return false for empty URL")
	}
}

func TestIsAvailableUnreachable(t *testing.T) {
	fp := NewFilecoinPinner("http://127.0.0.1:1", "test-token")
	if fp.IsAvailable() {
		t.Fatal("expected IsAvailable to return false for unreachable server")
	}
}

func TestUploadData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/upload" {
			t.Errorf("expected path /api/upload, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Filename") != "test.txt" {
			t.Errorf("expected X-Filename test.txt, got %s", r.Header.Get("X-Filename"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"cid": "QmUploadedCID",
		})
	}))
	defer server.Close()

	fp := NewFilecoinPinner(server.URL, "test-token")
	cid, err := fp.UploadData("test.txt", []byte("hello world"))
	if err != nil {
		t.Fatalf("UploadData failed: %v", err)
	}
	if cid != "QmUploadedCID" {
		t.Fatalf("expected CID QmUploadedCID, got %s", cid)
	}
}

// Verify the format string import is used
func TestFmtUsage(t *testing.T) {
	_ = fmt.Sprintf("test %s", "value")
}
