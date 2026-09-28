package content

import (
	"testing"
)

func TestCatalogCRUD(t *testing.T) {
	cat := NewCatalog()

	entry := ContentEntry{
		ID:         "c-001",
		Title:      "Test Video",
		Type:       ContentVideo,
		Size:       1024,
		Uploader:   "alice",
		ManifestID: "man-001",
		Access:     "public",
	}

	// Add
	if err := cat.Add(entry); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Get
	got, err := cat.Get("c-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Test Video" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Video")
	}
	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}

	// Duplicate add
	if err := cat.Add(entry); err != ErrAlreadyExists {
		t.Errorf("duplicate Add error = %v, want ErrAlreadyExists", err)
	}

	// Update
	updated := ContentEntry{
		Title:      "Updated Video",
		Type:       ContentVideo,
		Size:       2048,
		Uploader:   "alice",
		ManifestID: "man-001",
		Access:     "private",
	}
	if err := cat.Update("c-001", updated); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err = cat.Get("c-001")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Title != "Updated Video" {
		t.Errorf("Title after update = %q, want %q", got.Title, "Updated Video")
	}
	if got.Version != 2 {
		t.Errorf("Version after update = %d, want 2", got.Version)
	}

	// Update non-existent
	if err := cat.Update("nope", updated); err != ErrNotFound {
		t.Errorf("Update non-existent error = %v, want ErrNotFound", err)
	}

	// Delete
	if err := cat.Delete("c-001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := cat.Get("c-001"); err != ErrNotFound {
		t.Errorf("Get after delete error = %v, want ErrNotFound", err)
	}

	// Delete non-existent
	if err := cat.Delete("nope"); err != ErrNotFound {
		t.Errorf("Delete non-existent error = %v, want ErrNotFound", err)
	}
}

func TestCatalogList(t *testing.T) {
	cat := NewCatalog()

	entries := []ContentEntry{
		{ID: "v1", Title: "Video 1", Type: ContentVideo, Uploader: "alice", Access: "public"},
		{ID: "v2", Title: "Video 2", Type: ContentVideo, Uploader: "bob", Access: "private"},
		{ID: "a1", Title: "Audio 1", Type: ContentAudio, Uploader: "alice", Access: "public"},
		{ID: "d1", Title: "Doc 1", Type: ContentDocument, Uploader: "charlie", Access: "paid"},
	}
	for _, e := range entries {
		if err := cat.Add(e); err != nil {
			t.Fatal(err)
		}
	}

	// Filter by type: video
	videos := cat.List(CatalogFilter{Type: ContentVideo})
	if len(videos) != 2 {
		t.Errorf("video count = %d, want 2", len(videos))
	}

	// Filter by uploader: alice
	alice := cat.List(CatalogFilter{Uploader: "alice"})
	if len(alice) != 2 {
		t.Errorf("alice count = %d, want 2", len(alice))
	}

	// Filter by access: public
	public := cat.List(CatalogFilter{Access: "public"})
	if len(public) != 2 {
		t.Errorf("public count = %d, want 2", len(public))
	}

	// Combined filter
	aliceVideos := cat.List(CatalogFilter{Type: ContentVideo, Uploader: "alice"})
	if len(aliceVideos) != 1 {
		t.Errorf("alice videos count = %d, want 1", len(aliceVideos))
	}

	// Limit
	limited := cat.List(CatalogFilter{Limit: 2})
	if len(limited) != 2 {
		t.Errorf("limited count = %d, want 2", len(limited))
	}

	// Offset beyond range
	empty := cat.List(CatalogFilter{Offset: 100})
	if len(empty) != 0 {
		t.Errorf("offset beyond range count = %d, want 0", len(empty))
	}

	// No filter: all
	all := cat.List(CatalogFilter{})
	if len(all) != 4 {
		t.Errorf("all count = %d, want 4", len(all))
	}
}

func TestCatalogSearch(t *testing.T) {
	cat := NewCatalog()

	entries := []ContentEntry{
		{ID: "1", Title: "Introduction to Go Programming"},
		{ID: "2", Title: "Advanced Go Concurrency"},
		{ID: "3", Title: "Python Data Science"},
		{ID: "4", Title: "Rust for Systems Programming"},
	}
	for _, e := range entries {
		if err := cat.Add(e); err != nil {
			t.Fatal(err)
		}
	}

	// Search for "go" (case-insensitive)
	goResults := cat.Search("go")
	if len(goResults) != 2 {
		t.Errorf("search 'go' count = %d, want 2", len(goResults))
	}

	// Search for "programming"
	progResults := cat.Search("programming")
	if len(progResults) != 2 {
		t.Errorf("search 'programming' count = %d, want 2", len(progResults))
	}

	// Search for "rust"
	rustResults := cat.Search("rust")
	if len(rustResults) != 1 {
		t.Errorf("search 'rust' count = %d, want 1", len(rustResults))
	}

	// Search for non-existent
	none := cat.Search("xyzzy")
	if len(none) != 0 {
		t.Errorf("search 'xyzzy' count = %d, want 0", len(none))
	}
}

func TestCatalogVersioning(t *testing.T) {
	cat := NewCatalog()

	entry := ContentEntry{
		ID:     "ver-001",
		Title:  "Version Test v1",
		Type:   ContentDocument,
		Access: "public",
	}
	if err := cat.Add(entry); err != nil {
		t.Fatal(err)
	}

	got, _ := cat.Get("ver-001")
	if got.Version != 1 {
		t.Fatalf("initial version = %d, want 1", got.Version)
	}
	v1CreatedAt := got.CreatedAt

	// Update to v2
	entry.Title = "Version Test v2"
	if err := cat.Update("ver-001", entry); err != nil {
		t.Fatal(err)
	}

	got, _ = cat.Get("ver-001")
	if got.Version != 2 {
		t.Errorf("version after 1st update = %d, want 2", got.Version)
	}
	if got.CreatedAt != v1CreatedAt {
		t.Error("CreatedAt should not change on update")
	}

	// Update to v3
	entry.Title = "Version Test v3"
	if err := cat.Update("ver-001", entry); err != nil {
		t.Fatal(err)
	}

	got, _ = cat.Get("ver-001")
	if got.Version != 3 {
		t.Errorf("version after 2nd update = %d, want 3", got.Version)
	}
}
