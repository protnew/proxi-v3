package main

import (
	"bytes"
	"io"
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
