package main

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn/storage"
	"github.com/unkillable-messenger/vpn/store"
)

func TestMediaStream(t *testing.T) {
	// Setup in-memory db
	testDb, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	defer testDb.Close()

	// Setup local storage provider for tests
	dataDir := t.TempDir()
	storageProvider, _ = storage.NewLocalStore(dataDir)

	// 1. Mocking the file in storage (used for Streaming)
	chunkHash := "test-chunk-123"
	content := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	_, _, err = storageProvider.SaveFile(chunkHash, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("save file to storage: %v", err)
	}

	manifestID := "test-manifest-abc"

	// 2. Mocking store records (ContentManifest & ContentCatalog)
	err = testDb.SaveContentManifest(store.ContentManifest{
		ID:          manifestID,
		OwnerNpub:   "test-owner",
		ContentType: "video/mp4", // Enables Range requests in handleMediaStream
		Metadata:    `{"filename":"test.mp4"}`,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("save manifest: %v", err)
	}

	err = testDb.SaveContentCatalog(store.ContentCatalog{
		ID:           "test-catalog-abc",
		ManifestID:   manifestID,
		ChunkHash:    chunkHash,
		Size:         int64(len(content)),
		Availability: "local",
	})
	if err != nil {
		t.Fatalf("save catalog: %v", err)
	}

	// Замоккай store.Stream (Mock store.Stream struct in DB)
	err = testDb.SaveStream(store.Stream{
		ID:        manifestID,
		Creator:   "test-owner",
		Title:     "Mock Stream",
		Status:    "active",
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("save stream mock: %v", err)
	}

	srv := &Server{
		db: testDb,
	}

	t.Run("Full Response", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/media/"+manifestID+"/stream", nil)
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		
		// Map directly via handler like the router would
		srv.handleMediaGet(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
			body, _ := io.ReadAll(rr.Body)
			t.Logf("Response body: %s", body)
		}

		if !bytes.Equal(rr.Body.Bytes(), content) {
			t.Errorf("expected body %q, got %q", content, rr.Body.Bytes())
		}
	})

	t.Run("Range Request", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/media/"+manifestID+"/stream", nil)
		if err != nil {
			t.Fatal(err)
		}
		// Request bytes 5 to 14: "56789ABCDE"
		req.Header.Set("Range", "bytes=5-14")
		rr := httptest.NewRecorder()
		
		srv.handleMediaGet(rr, req)

		if rr.Code != http.StatusPartialContent { // 206
			t.Errorf("expected 206 Partial Content, got %d", rr.Code)
		}

		expectedRange := []byte("56789ABCDE")
		if !bytes.Equal(rr.Body.Bytes(), expectedRange) {
			t.Errorf("expected range body %q, got %q", expectedRange, rr.Body.Bytes())
		}
	})
	
	t.Run("NotFound", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/media/non-existent/stream", nil)
		rr := httptest.NewRecorder()
		srv.handleMediaGet(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}

type failStorage struct{}
func (f *failStorage) SaveFile(hash string, r io.Reader) (string, int64, error) {
	return "", 0, io.ErrUnexpectedEOF
}
func (f *failStorage) GetFile(hash string) (io.ReadCloser, error) {
	return nil, io.ErrUnexpectedEOF
}
func (f *failStorage) DeleteFile(hash string) error {
	return nil
}

type zeroReader struct{}

func (z zeroReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestMediaUpload(t *testing.T) {
	testDb, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	defer testDb.Close()

	dataDir := t.TempDir()
	originalProvider := storageProvider
	defer func() { storageProvider = originalProvider }()
	storageProvider, _ = storage.NewLocalStore(dataDir)

	srv := &Server{
		db: testDb,
	}

	t.Run("Method Not Allowed", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/media/upload", nil)
		rr := httptest.NewRecorder()
		srv.handleMediaUpload(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rr.Code)
		}
	})

	t.Run("Parse Error", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/media/upload", bytes.NewReader([]byte("bad body")))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
		rr := httptest.NewRecorder()
		srv.handleMediaUpload(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("No File Provided", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("npub", "test-user")
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/media/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		srv.handleMediaUpload(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("File Too Large", func(t *testing.T) {
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		
		go func() {
			part, _ := writer.CreateFormFile("file", "large.txt")
			// write 50MB + 1 byte
			io.Copy(part, io.LimitReader(zeroReader{}, (50<<20)+1))
			writer.Close()
			pw.Close()
		}()

		req, _ := http.NewRequest("POST", "/api/media/upload", pr)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		
		rr := httptest.NewRecorder()
		srv.handleMediaUpload(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("Success Upload", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.txt")
		content := []byte("hello world file content")
		part.Write(content)
		writer.WriteField("npub", "spoof-attempt-npub") // P16: form field ignored, JWT ctx wins
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/media/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), "npub", "test-user-npub"))

		rr := httptest.NewRecorder()
		srv.handleMediaUpload(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", rr.Code)
			t.Logf("Response: %s", rr.Body.String())
		}

		// check response json
		if !bytes.Contains(rr.Body.Bytes(), []byte("manifest_id")) {
			t.Errorf("expected manifest_id in response")
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("chunk_hash")) {
			t.Errorf("expected chunk_hash in response")
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("test.txt")) {
			t.Errorf("expected filename test.txt in response")
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("test-user-npub")) {
			t.Errorf("expected npub test-user-npub in response")
		}
	})

	t.Run("Storage Provider Nil", func(t *testing.T) {
		storageProvider = nil
		defer func() { storageProvider, _ = storage.NewLocalStore(dataDir) }()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte("fail content"))
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/media/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		srv.handleMediaUpload(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rr.Code)
		}
	})

	t.Run("Storage Save Error", func(t *testing.T) {
		storageProvider = &failStorage{}
		defer func() { storageProvider, _ = storage.NewLocalStore(dataDir) }()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte("fail content"))
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/media/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		srv.handleMediaUpload(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rr.Code)
		}
	})

	t.Run("DB Save Error", func(t *testing.T) {
		failDb, _ := store.NewStore(":memory:")
		failDb.Close()

		failSrv := &Server{db: failDb}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte("fail content"))
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/media/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		failSrv.handleMediaUpload(rr, req)
		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", rr.Code)
		}
	})
}

func TestMediaGet(t *testing.T) {
	testDb, _ := store.NewStore(":memory:")
	defer testDb.Close()
	dataDir := t.TempDir()
	originalProvider := storageProvider
	defer func() { storageProvider = originalProvider }()
	storageProvider, _ = storage.NewLocalStore(dataDir)
	srv := &Server{db: testDb}

	chunkHash := "test-chunk-get"
	content := []byte("get file data")
	storageProvider.SaveFile(chunkHash, bytes.NewReader(content))

	manifestID := "test-manifest-get"
	testDb.SaveContentManifest(store.ContentManifest{
		ID:          manifestID,
		ContentType: "image/png",
		Metadata:    `{"filename":"test.png"}`,
	})
	testDb.SaveContentCatalog(store.ContentCatalog{
		ManifestID: manifestID,
		ChunkHash:  chunkHash,
		Size:       int64(len(content)),
	})

	t.Run("OPTIONS", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", "/api/media/"+manifestID, nil)
		rr := httptest.NewRecorder()
		srv.handleMediaGet(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rr.Code)
		}
	})
	t.Run("Missing ID", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/media/", nil)
		rr := httptest.NewRecorder()
		srv.handleMediaGet(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})
	t.Run("Method Not Allowed", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/media/"+manifestID, nil)
		rr := httptest.NewRecorder()
		srv.handleMediaGet(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rr.Code)
		}
	})
	t.Run("Manifest Not Found", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/media/missing", nil)
		rr := httptest.NewRecorder()
		srv.handleMediaGet(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
	t.Run("Success GET image", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/media/"+manifestID, nil)
		rr := httptest.NewRecorder()
		srv.handleMediaGet(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestMediaStreamExtra(t *testing.T) {
	testDb, _ := store.NewStore(":memory:")
	defer testDb.Close()
	dataDir := t.TempDir()
	originalProvider := storageProvider
	defer func() { storageProvider = originalProvider }()
	storageProvider, _ = storage.NewLocalStore(dataDir)
	srv := &Server{db: testDb}

	chunkHash := "test-chunk-stream"
	content := []byte("stream file data")
	storageProvider.SaveFile(chunkHash, bytes.NewReader(content))

	manifestID := "test-manifest-stream"
	testDb.SaveContentManifest(store.ContentManifest{
		ID:          manifestID,
		ContentType: "video/mp4",
		Metadata:    `{"filename":"test.mp4"}`,
	})
	testDb.SaveContentCatalog(store.ContentCatalog{
		ManifestID: manifestID,
		ChunkHash:  chunkHash,
		Size:       int64(len(content)),
	})

	t.Run("Method Not Allowed", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/media/"+manifestID+"/stream", nil)
		rr := httptest.NewRecorder()
		srv.handleMediaStream(rr, req, manifestID)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rr.Code)
		}
	})
	t.Run("Manifest Not Found", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/media/missing/stream", nil)
		rr := httptest.NewRecorder()
		srv.handleMediaStream(rr, req, "missing")
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
	t.Run("Catalog Not Found", func(t *testing.T) {
		testDb.SaveContentManifest(store.ContentManifest{ID: "missing-catalog"})
		req, _ := http.NewRequest("GET", "/api/media/missing-catalog/stream", nil)
		rr := httptest.NewRecorder()
		srv.handleMediaStream(rr, req, "missing-catalog")
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
	t.Run("HEAD request", func(t *testing.T) {
		req, _ := http.NewRequest("HEAD", "/api/media/"+manifestID+"/stream", nil)
		rr := httptest.NewRecorder()
		srv.handleMediaStream(rr, req, manifestID)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}
