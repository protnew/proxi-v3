package content

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "content_test.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := EnsureContentTables(db); err != nil {
		t.Fatalf("EnsureContentTables: %v", err)
	}
	return db
}

func TestEnsureContentTables_Idempotent(t *testing.T) {
	db := newTestDB(t)
	// Calling again must not error.
	if err := EnsureContentTables(db); err != nil {
		t.Fatalf("second EnsureContentTables: %v", err)
	}
}

func TestManifestCRUDRoundtrip(t *testing.T) {
	db := newTestDB(t)

	original := &StoredManifest{
		ID:                 "man-crud-001",
		FileName:           "movie.mp4",
		FileSize:           4096,
		ChunkSize:          1024,
		TotalChunks:        4,
		ErasureK:           3,
		ErasureN:           5,
		EncryptedMasterKey: []byte("0123456789abcdef"),
		Chunks: []ChunkRef{
			{Hash: "hash0", EncryptedHash: "enchash0", Size: 1040},
			{Hash: "hash1", EncryptedHash: "enchash1", Size: 1040},
			{Hash: "hash2", EncryptedHash: "enchash2", Size: 1040},
			{Hash: "hash3", EncryptedHash: "enchash3", Size: 1040},
		},
		CreatedAt: 1700000000,
		Payload:   []byte("encrypted-payload-blob"),
	}

	if err := SaveManifest(db, original); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	loaded, err := LoadManifest(db, "man-crud-001")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if loaded.ID != original.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, original.ID)
	}
	if loaded.FileName != original.FileName {
		t.Errorf("FileName = %q, want %q", loaded.FileName, original.FileName)
	}
	if loaded.FileSize != original.FileSize {
		t.Errorf("FileSize = %d, want %d", loaded.FileSize, original.FileSize)
	}
	if loaded.ChunkSize != original.ChunkSize {
		t.Errorf("ChunkSize = %d, want %d", loaded.ChunkSize, original.ChunkSize)
	}
	if loaded.TotalChunks != original.TotalChunks {
		t.Errorf("TotalChunks = %d, want %d", loaded.TotalChunks, original.TotalChunks)
	}
	if loaded.ErasureK != original.ErasureK || loaded.ErasureN != original.ErasureN {
		t.Errorf("Erasure = (%d,%d), want (%d,%d)", loaded.ErasureK, loaded.ErasureN, original.ErasureK, original.ErasureN)
	}
	if !bytes.Equal(loaded.EncryptedMasterKey, original.EncryptedMasterKey) {
		t.Errorf("EncryptedMasterKey mismatch")
	}
	if !bytes.Equal(loaded.Payload, original.Payload) {
		t.Errorf("Payload mismatch")
	}
	if loaded.CreatedAt != original.CreatedAt {
		t.Errorf("CreatedAt = %d, want %d", loaded.CreatedAt, original.CreatedAt)
	}
	if len(loaded.Chunks) != len(original.Chunks) {
		t.Fatalf("Chunks len = %d, want %d", len(loaded.Chunks), len(original.Chunks))
	}
	for i, c := range loaded.Chunks {
		if c != original.Chunks[i] {
			t.Errorf("chunk %d mismatch: %+v vs %+v", i, c, original.Chunks[i])
		}
	}

	// Update (replace) and reload.
	original.FileName = "renamed.mp4"
	original.FileSize = 8192
	if err := SaveManifest(db, original); err != nil {
		t.Fatalf("SaveManifest update: %v", err)
	}
	updated, err := LoadManifest(db, "man-crud-001")
	if err != nil {
		t.Fatalf("LoadManifest after update: %v", err)
	}
	if updated.FileName != "renamed.mp4" {
		t.Errorf("FileName after update = %q", updated.FileName)
	}
	if updated.FileSize != 8192 {
		t.Errorf("FileSize after update = %d, want 8192", updated.FileSize)
	}

	// Delete.
	if err := DeleteManifest(db, "man-crud-001"); err != nil {
		t.Fatalf("DeleteManifest: %v", err)
	}
	if _, err := LoadManifest(db, "man-crud-001"); err == nil {
		t.Fatal("expected error loading deleted manifest")
	}
}

func TestManifestLoadMissing(t *testing.T) {
	db := newTestDB(t)
	_, err := LoadManifest(db, "does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestManifestSaveNilBlobs(t *testing.T) {
	db := newTestDB(t)
	m := &StoredManifest{
		ID:          "man-nil",
		FileName:    "empty.bin",
		FileSize:    0,
		TotalChunks: 0,
		Chunks:      []ChunkRef{},
		CreatedAt:   1700000000,
		// EncryptedMasterKey and Payload left nil.
	}
	if err := SaveManifest(db, m); err != nil {
		t.Fatalf("SaveManifest with nil blobs: %v", err)
	}
	loaded, err := LoadManifest(db, "man-nil")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(loaded.EncryptedMasterKey) != 0 {
		t.Errorf("expected empty EncryptedMasterKey, got %d bytes", len(loaded.EncryptedMasterKey))
	}
	if len(loaded.Payload) != 0 {
		t.Errorf("expected empty Payload, got %d bytes", len(loaded.Payload))
	}
	if len(loaded.Chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(loaded.Chunks))
	}
}

func TestCatalogEntryCRUDRoundtrip(t *testing.T) {
	db := newTestDB(t)

	entry := &ContentEntry{
		ID:         "cat-crud-001",
		Title:      "Survival Video",
		Type:       ContentVideo,
		Size:       1048576,
		Uploader:   "alice",
		ManifestID: "man-crud-001",
		Access:     "public",
		Version:    1,
		CreatedAt:  1700000000,
		UpdatedAt:  1700000001,
	}

	if err := SaveCatalogEntry(db, entry); err != nil {
		t.Fatalf("SaveCatalogEntry: %v", err)
	}

	loaded, err := LoadCatalogEntry(db, "cat-crud-001")
	if err != nil {
		t.Fatalf("LoadCatalogEntry: %v", err)
	}
	if loaded.ID != entry.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, entry.ID)
	}
	if loaded.Title != entry.Title {
		t.Errorf("Title = %q, want %q", loaded.Title, entry.Title)
	}
	if loaded.Type != entry.Type {
		t.Errorf("Type = %q, want %q", loaded.Type, entry.Type)
	}
	if loaded.Size != entry.Size {
		t.Errorf("Size = %d, want %d", loaded.Size, entry.Size)
	}
	if loaded.Uploader != entry.Uploader {
		t.Errorf("Uploader = %q, want %q", loaded.Uploader, entry.Uploader)
	}
	if loaded.ManifestID != entry.ManifestID {
		t.Errorf("ManifestID = %q, want %q", loaded.ManifestID, entry.ManifestID)
	}
	if loaded.Access != entry.Access {
		t.Errorf("Access = %q, want %q", loaded.Access, entry.Access)
	}
	if loaded.Version != entry.Version {
		t.Errorf("Version = %d, want %d", loaded.Version, entry.Version)
	}

	// Update.
	entry.Title = "Renamed Video"
	entry.Access = "private"
	entry.Version = 2
	if err := SaveCatalogEntry(db, entry); err != nil {
		t.Fatalf("SaveCatalogEntry update: %v", err)
	}
	updated, err := LoadCatalogEntry(db, "cat-crud-001")
	if err != nil {
		t.Fatalf("LoadCatalogEntry after update: %v", err)
	}
	if updated.Title != "Renamed Video" {
		t.Errorf("Title after update = %q", updated.Title)
	}
	if updated.Access != "private" {
		t.Errorf("Access after update = %q", updated.Access)
	}
	if updated.Version != 2 {
		t.Errorf("Version after update = %d, want 2", updated.Version)
	}

	// Delete.
	if err := DeleteCatalogEntry(db, "cat-crud-001"); err != nil {
		t.Fatalf("DeleteCatalogEntry: %v", err)
	}
	if _, err := LoadCatalogEntry(db, "cat-crud-001"); err == nil {
		t.Fatal("expected error loading deleted entry")
	}
}

func TestListCatalogEntries_Pagination(t *testing.T) {
	db := newTestDB(t)

	entries := []*ContentEntry{
		{ID: "c01", Title: "A", Type: ContentVideo, Uploader: "alice", Access: "public", Version: 1, CreatedAt: 1, UpdatedAt: 1},
		{ID: "c02", Title: "B", Type: ContentVideo, Uploader: "alice", Access: "public", Version: 1, CreatedAt: 2, UpdatedAt: 2},
		{ID: "c03", Title: "C", Type: ContentAudio, Uploader: "bob", Access: "public", Version: 1, CreatedAt: 3, UpdatedAt: 3},
		{ID: "c04", Title: "D", Type: ContentDocument, Uploader: "bob", Access: "private", Version: 1, CreatedAt: 4, UpdatedAt: 4},
		{ID: "c05", Title: "E", Type: ContentImage, Uploader: "charlie", Access: "paid", Version: 1, CreatedAt: 5, UpdatedAt: 5},
	}
	for _, e := range entries {
		if err := SaveCatalogEntry(db, e); err != nil {
			t.Fatalf("SaveCatalogEntry %s: %v", e.ID, err)
		}
	}

	// All entries, ordered by ID.
	all, err := ListCatalogEntries(db, CatalogFilter{})
	if err != nil {
		t.Fatalf("ListCatalogEntries all: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("all count = %d, want 5", len(all))
	}

	// Filter by uploader.
	alice, err := ListCatalogEntries(db, CatalogFilter{Uploader: "alice"})
	if err != nil {
		t.Fatalf("ListCatalogEntries alice: %v", err)
	}
	if len(alice) != 2 {
		t.Errorf("alice count = %d, want 2", len(alice))
	}

	// Filter by type.
	videos, err := ListCatalogEntries(db, CatalogFilter{Type: ContentVideo})
	if err != nil {
		t.Fatalf("ListCatalogEntries videos: %v", err)
	}
	if len(videos) != 2 {
		t.Errorf("videos count = %d, want 2", len(videos))
	}

	// Filter by access.
	public, err := ListCatalogEntries(db, CatalogFilter{Access: "public"})
	if err != nil {
		t.Fatalf("ListCatalogEntries public: %v", err)
	}
	if len(public) != 3 {
		t.Errorf("public count = %d, want 3", len(public))
	}

	// Pagination: limit 2, offset 0.
	page1, err := ListCatalogEntries(db, CatalogFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("ListCatalogEntries page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 count = %d, want 2", len(page1))
	}
	if page1[0].ID != "c01" || page1[1].ID != "c02" {
		t.Errorf("page1 IDs = %s,%s, want c01,c02", page1[0].ID, page1[1].ID)
	}

	// Pagination: limit 2, offset 2.
	page2, err := ListCatalogEntries(db, CatalogFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListCatalogEntries page2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2 count = %d, want 2", len(page2))
	}
	if page2[0].ID != "c03" || page2[1].ID != "c04" {
		t.Errorf("page2 IDs = %s,%s, want c03,c04", page2[0].ID, page2[1].ID)
	}

	// Pagination: offset beyond range.
	empty, err := ListCatalogEntries(db, CatalogFilter{Limit: 10, Offset: 100})
	if err != nil {
		t.Fatalf("ListCatalogEntries empty: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("empty count = %d, want 0", len(empty))
	}
}
