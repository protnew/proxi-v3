package ipfs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient_CustomURLs(t *testing.T) {
	c := NewClient("https://custom-gateway.com/ipfs/", "http://1.2.3.4:5001")
	if c.gateway != "https://custom-gateway.com/ipfs/" {
		t.Errorf("expected custom gateway, got %q", c.gateway)
	}
	if c.apiURL != "http://1.2.3.4:5001" {
		t.Errorf("expected custom apiURL, got %q", c.apiURL)
	}
}

func TestUploadFile_WithMockDaemon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v0/add") {
			t.Errorf("expected /api/v0/add path, got %s", r.URL.Path)
		}
		// Verify multipart form data
		if r.Header.Get("Content-Type") == "" {
			t.Error("Content-Type header missing")
		}

		// Return IPFS-style response
		resp := map[string]string{
			"Name": "test.txt",
			"Hash": "QmXyz123FakeCID",
			"Size": "1234",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient("https://ipfs.io/ipfs/", server.URL)
	result, err := c.UploadFile("test.txt", []byte("hello world"))
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if result.CID != "QmXyz123FakeCID" {
		t.Errorf("expected CID QmXyz123FakeCID, got %q", result.CID)
	}
	if result.GatewayURL != "https://ipfs.io/ipfs/QmXyz123FakeCID" {
		t.Errorf("unexpected GatewayURL: %q", result.GatewayURL)
	}
	if result.Size != 1234 {
		t.Errorf("expected size 1234, got %d", result.Size)
	}
}

func TestUploadFile_APIServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	c := NewClient("", server.URL)
	_, err := c.UploadFile("test.txt", []byte("hello"))
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}

func TestUploadBytes_WithMockDaemon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"Name": "data",
			"Hash": "QmDataCID",
			"Size": "56",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient("", server.URL)
	result, err := c.UploadBytes([]byte("raw binary data"))
	if err != nil {
		t.Fatalf("UploadBytes: %v", err)
	}
	if result.CID != "QmDataCID" {
		t.Errorf("expected CID QmDataCID, got %q", result.CID)
	}
}

func TestDownloadFile_WithMockGateway(t *testing.T) {
	expectedContent := "downloaded file content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "QmTestCID") {
			t.Errorf("expected CID in URL path, got %s", r.URL.Path)
		}
		w.Write([]byte(expectedContent))
	}))
	defer server.Close()

	c := NewClient(server.URL+"/ipfs/", "http://127.0.0.1:5001")
	data, err := c.DownloadFile("QmTestCID")
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if string(data) != expectedContent {
		t.Errorf("expected %q, got %q", expectedContent, string(data))
	}
}

func TestDownloadFile_GatewayError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	c := NewClient(server.URL+"/ipfs/", "http://127.0.0.1:5001")
	_, err := c.DownloadFile("QmNotFound")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404, got: %v", err)
	}
}

func TestPinFile_WithMockDaemon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v0/pin/add") {
			t.Errorf("expected pin/add path, got %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "QmPinCID") {
			t.Errorf("expected CID in query, got %s", r.URL.RawQuery)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	c := NewClient("", server.URL)
	err := c.PinFile("QmPinCID")
	if err != nil {
		t.Fatalf("PinFile: %v", err)
	}
}

func TestPinFile_APIServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	c := NewClient("", server.URL)
	err := c.PinFile("QmSomeCID")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}

func TestIsAvailable_WithMockDaemon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v0/version" {
			w.WriteHeader(200)
			w.Write([]byte(`{"Version":"0.12.0"}`))
		}
	}))
	defer server.Close()

	c := NewClient("", server.URL)
	if !c.IsAvailable() {
		t.Error("expected daemon to be available")
	}
}

func TestIsAvailable_DaemonDown(t *testing.T) {
	c := NewClient("", "http://127.0.0.1:19999")
	if c.IsAvailable() {
		t.Error("expected daemon to be unavailable on port 19999")
	}
}

func TestDownloadFile_LargeContent(t *testing.T) {
	largeContent := strings.Repeat("x", 100000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(largeContent))
	}))
	defer server.Close()

	c := NewClient(server.URL+"/ipfs/", "")
	data, err := c.DownloadFile("QmLarge")
	if err != nil {
		t.Fatalf("DownloadFile large: %v", err)
	}
	if len(data) != 100000 {
		t.Errorf("expected 100000 bytes, got %d", len(data))
	}
}

func TestUploadFile_MultipartContent(t *testing.T) {
	var contentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		// Consume and verify the body is valid multipart
		r.ParseMultipartForm(10 << 20)
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("failed to get form file: %v", err)
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		buf := make([]byte, 100)
		n, _ := file.Read(buf)

		resp := map[string]string{
			"Name": "data.bin",
			"Hash": "QmBinary",
			"Size": fmt.Sprintf("%d", n),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient("", server.URL)
	testData := []byte("binary\x00content\xff\xfe")
	result, err := c.UploadFile("data.bin", testData)
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("expected multipart content type, got %q", contentType)
	}
	if result.CID != "QmBinary" {
		t.Errorf("expected CID QmBinary, got %q", result.CID)
	}
}
