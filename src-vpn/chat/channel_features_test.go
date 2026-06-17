package chat

import (
	"sync/atomic"
	"testing"
	"time"
)

// ===========================================================================
// Category CRUD tests
// ===========================================================================

// ---------------------------------------------------------------------------
// TestCategoryCRUD — create, read, update, delete
// ---------------------------------------------------------------------------

func TestCategoryCRUD(t *testing.T) {
	t.Parallel()

	cm := NewCategoryManager()

	// Create.
	cat := cm.CreateCategory("cat-1", "Work")
	if cat.ID != "cat-1" || cat.Name != "Work" {
		t.Fatalf("unexpected category: %+v", cat)
	}

	// Read.
	got, err := cm.GetCategory("cat-1")
	if err != nil {
		t.Fatalf("GetCategory error: %v", err)
	}
	if got.Name != "Work" {
		t.Fatalf("expected name Work, got %s", got.Name)
	}

	// Update.
	if err := cm.UpdateCategory("cat-1", "Work Updated"); err != nil {
		t.Fatalf("UpdateCategory error: %v", err)
	}
	got, _ = cm.GetCategory("cat-1")
	if got.Name != "Work Updated" {
		t.Fatalf("expected updated name, got %s", got.Name)
	}

	// Delete.
	if err := cm.DeleteCategory("cat-1"); err != nil {
		t.Fatalf("DeleteCategory error: %v", err)
	}
	if _, err := cm.GetCategory("cat-1"); err != ErrCategoryNotFound {
		t.Fatalf("expected ErrCategoryNotFound after delete, got %v", err)
	}

	// Delete non-existent.
	if err := cm.DeleteCategory("nope"); err != ErrCategoryNotFound {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
	}

	// Update non-existent.
	if err := cm.UpdateCategory("nope", "x"); err != ErrCategoryNotFound {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestCategoryChannels — add/remove channels, move between categories
// ---------------------------------------------------------------------------

func TestCategoryChannels(t *testing.T) {
	t.Parallel()

	cm := NewCategoryManager()
	cm.CreateCategory("work", "Work")
	cm.CreateCategory("personal", "Personal")

	// Add channels to "work".
	if err := cm.AddChannel("work", "ch-dev"); err != nil {
		t.Fatalf("AddChannel error: %v", err)
	}
	if err := cm.AddChannel("work", "ch-meetings"); err != nil {
		t.Fatalf("AddChannel error: %v", err)
	}

	channels, _ := cm.ListChannels("work")
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels in work, got %d", len(channels))
	}

	// CategoryOfChannel.
	if cat := cm.CategoryOfChannel("ch-dev"); cat != "work" {
		t.Fatalf("expected ch-dev in work, got %s", cat)
	}

	// Move ch-dev from work to personal.
	if err := cm.AddChannel("personal", "ch-dev"); err != nil {
		t.Fatalf("move error: %v", err)
	}
	if cat := cm.CategoryOfChannel("ch-dev"); cat != "personal" {
		t.Fatalf("expected ch-dev moved to personal, got %s", cat)
	}
	workChans, _ := cm.ListChannels("work")
	if len(workChans) != 1 || workChans[0] != "ch-meetings" {
		t.Fatalf("work should have only ch-meetings after move, got %v", workChans)
	}

	// Remove ch-meetings.
	if err := cm.RemoveChannel("ch-meetings"); err != nil {
		t.Fatalf("RemoveChannel error: %v", err)
	}
	workChans, _ = cm.ListChannels("work")
	if len(workChans) != 0 {
		t.Fatalf("expected 0 channels in work after remove, got %d", len(workChans))
	}

	// Remove non-assigned channel.
	if err := cm.RemoveChannel("ch-ghost"); err != ErrChannelNotInCategory {
		t.Fatalf("expected ErrChannelNotInCategory, got %v", err)
	}

	// AddChannel to non-existent category.
	if err := cm.AddChannel("nonexistent", "ch-x"); err != ErrCategoryNotFound {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestCategoryDeleteUnassignsChannels — deleting a category frees its channels
// ---------------------------------------------------------------------------

func TestCategoryDeleteUnassignsChannels(t *testing.T) {
	t.Parallel()

	cm := NewCategoryManager()
	cm.CreateCategory("temp", "Temporary")
	_ = cm.AddChannel("temp", "ch-a")
	_ = cm.AddChannel("temp", "ch-b")

	if err := cm.DeleteCategory("temp"); err != nil {
		t.Fatalf("DeleteCategory error: %v", err)
	}
	// Channels should no longer be assigned to any category.
	if cat := cm.CategoryOfChannel("ch-a"); cat != "" {
		t.Fatalf("expected ch-a unassigned, got %s", cat)
	}
	if cat := cm.CategoryOfChannel("ch-b"); cat != "" {
		t.Fatalf("expected ch-b unassigned, got %s", cat)
	}
}

// ---------------------------------------------------------------------------
// TestListCategories — multiple categories
// ---------------------------------------------------------------------------

func TestListCategories(t *testing.T) {
	t.Parallel()

	cm := NewCategoryManager()
	cm.CreateCategory("c1", "One")
	cm.CreateCategory("c2", "Two")
	cm.CreateCategory("c3", "Three")

	list := cm.ListCategories()
	if len(list) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(list))
	}
	if cm.CategoryCount() != 3 {
		t.Fatalf("expected count 3, got %d", cm.CategoryCount())
	}
}

// ===========================================================================
// Subscriber Count Cache tests
// ===========================================================================

// ---------------------------------------------------------------------------
// TestSubCountCacheLazyUpdate — cache fetches from source on miss/expiry
// ---------------------------------------------------------------------------

func TestSubCountCacheLazyUpdate(t *testing.T) {
	t.Parallel()

	var fetchCount int32
	source := func(channelID string) int64 {
		atomic.AddInt32(&fetchCount, 1)
		return 42
	}

	cache := NewSubscriberCountCache(5*time.Minute, source)

	// First GetCount — should fetch from source.
	if c := cache.GetCount("ch1"); c != 42 {
		t.Fatalf("expected 42, got %d", c)
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Fatalf("expected 1 source fetch, got %d", fetchCount)
	}

	// Second GetCount — cache hit, no new fetch.
	if c := cache.GetCount("ch1"); c != 42 {
		t.Fatalf("expected 42, got %d", c)
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Fatalf("expected still 1 fetch (cache hit), got %d", fetchCount)
	}

	// Invalidate — next GetCount should fetch again.
	cache.Invalidate("ch1")
	if c := cache.GetCount("ch1"); c != 42 {
		t.Fatalf("expected 42, got %d", c)
	}
	if atomic.LoadInt32(&fetchCount) != 2 {
		t.Fatalf("expected 2 fetches after invalidate, got %d", fetchCount)
	}
}

// ---------------------------------------------------------------------------
// TestSubCountCacheExpiry — stale entry triggers lazy refresh
// ---------------------------------------------------------------------------

func TestSubCountCacheExpiry(t *testing.T) {
	t.Parallel()

	var fetchCount int32
	source := func(channelID string) int64 {
		n := atomic.AddInt32(&fetchCount, 1)
		return int64(n) // returns 1, 2, 3... on each fetch
	}

	cache := NewSubscriberCountCache(50*time.Millisecond, source)

	// Initial fetch.
	if c := cache.GetCount("ch-exp"); c != 1 {
		t.Fatalf("expected 1, got %d", c)
	}
	// Still fresh.
	if c := cache.GetCount("ch-exp"); c != 1 {
		t.Fatalf("expected 1 (cached), got %d", c)
	}

	// Wait for expiry.
	time.Sleep(60 * time.Millisecond)
	// Now stale — should re-fetch and return 2.
	if c := cache.GetCount("ch-exp"); c != 2 {
		t.Fatalf("expected 2 after expiry, got %d", c)
	}
	if atomic.LoadInt32(&fetchCount) != 2 {
		t.Fatalf("expected 2 fetches after expiry, got %d", fetchCount)
	}
}

// ---------------------------------------------------------------------------
// TestSubCountCacheSetCount — manual set bypasses source
// ---------------------------------------------------------------------------

func TestSubCountCacheSetCount(t *testing.T) {
	t.Parallel()

	var fetchCount int32
	source := func(channelID string) int64 {
		atomic.AddInt32(&fetchCount, 1)
		return 0
	}

	cache := NewSubscriberCountCache(5*time.Minute, source)
	cache.SetCount("ch-set", 999)

	if c := cache.GetCount("ch-set"); c != 999 {
		t.Fatalf("expected 999, got %d", c)
	}
	if atomic.LoadInt32(&fetchCount) != 0 {
		t.Fatalf("expected 0 source fetches (manual set), got %d", fetchCount)
	}
	if !cache.Has("ch-set") {
		t.Fatal("expected cache to have entry for ch-set")
	}
}

// ---------------------------------------------------------------------------
// TestSubCountCacheInvalidateAll — invalidate all entries
// ---------------------------------------------------------------------------

func TestSubCountCacheInvalidateAll(t *testing.T) {
	t.Parallel()

	var fetchCount int32
	source := func(channelID string) int64 {
		atomic.AddInt32(&fetchCount, 1)
		return 100
	}

	cache := NewSubscriberCountCache(5*time.Minute, source)
	cache.GetCount("ch-a")
	cache.GetCount("ch-b")
	if atomic.LoadInt32(&fetchCount) != 2 {
		t.Fatalf("expected 2 fetches, got %d", fetchCount)
	}

	// Invalidate all, then re-fetch both.
	cache.InvalidateAll()
	cache.GetCount("ch-a")
	cache.GetCount("ch-b")
	if atomic.LoadInt32(&fetchCount) != 4 {
		t.Fatalf("expected 4 fetches after InvalidateAll, got %d", fetchCount)
	}
}

// ---------------------------------------------------------------------------
// TestSubCountCacheNoSource — returns 0 or last value when source is nil
// ---------------------------------------------------------------------------

func TestSubCountCacheNoSource(t *testing.T) {
	t.Parallel()

	cache := NewSubscriberCountCache(5*time.Minute, nil)

	// No entry, no source → 0.
	if c := cache.GetCount("ch-none"); c != 0 {
		t.Fatalf("expected 0 with no source, got %d", c)
	}

	// Manual set then read.
	cache.SetCount("ch-manual", 55)
	if c := cache.GetCount("ch-manual"); c != 55 {
		t.Fatalf("expected 55, got %d", c)
	}

	// Invalidate then read with no source → returns last value.
	cache.Invalidate("ch-manual")
	if c := cache.GetCount("ch-manual"); c != 55 {
		t.Fatalf("expected 55 (last value, no source), got %d", c)
	}
}
