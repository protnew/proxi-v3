package chat

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestParseOGTags — parse various og:tag formats from sample HTML
// ---------------------------------------------------------------------------

func TestParseOGTags(t *testing.T) {
	t.Parallel()

	html := `<!DOCTYPE html>
<html><head>
  <title>Fallback Title</title>
  <meta property="og:title" content="Example Page" />
  <meta property="og:description" content="A description &amp; more">
  <meta property="og:image" content='https://example.com/img.png'>
  <meta property="og:url" content="https://example.com/page">
  <meta name="twitter:card" content="summary">
  <meta property="og:site_name" content="ExampleSite">
</head><body>hello</body></html>`

	tags := ParseOGTags(html)

	if tags["title"] != "Example Page" {
		t.Errorf("og:title = %q, want %q", tags["title"], "Example Page")
	}
	if tags["description"] != "A description & more" {
		t.Errorf("og:description = %q, want %q", tags["description"], "A description & more")
	}
	if tags["image"] != "https://example.com/img.png" {
		t.Errorf("og:image = %q, want %q", tags["image"], "https://example.com/img.png")
	}
	if tags["url"] != "https://example.com/page" {
		t.Errorf("og:url = %q, want %q", tags["url"], "https://example.com/page")
	}
	if tags["site_name"] != "ExampleSite" {
		t.Errorf("og:site_name = %q, want %q", tags["site_name"], "ExampleSite")
	}
	// twitter:card is NOT an og: tag, should not appear.
	if _, ok := tags["twitter:card"]; ok {
		t.Error("twitter:card should not be parsed as og: tag")
	}
}

// ---------------------------------------------------------------------------
// TestParseOGTagsContentBeforeProperty — attribute order independence
// ---------------------------------------------------------------------------

func TestParseOGTagsContentBeforeProperty(t *testing.T) {
	t.Parallel()

	// content appears before property in some sites.
	html := `<meta content="Reversed Order" property="og:title">`
	tags := ParseOGTags(html)
	if tags["title"] != "Reversed Order" {
		t.Errorf("og:title = %q, want %q", tags["title"], "Reversed Order")
	}
}

// ---------------------------------------------------------------------------
// TestParseOGTagsNoTags — empty/no og:tags returns empty map
// ---------------------------------------------------------------------------

func TestParseOGTagsNoTags(t *testing.T) {
	t.Parallel()

	tags := ParseOGTags("<html><body>no meta here</body></html>")
	if len(tags) != 0 {
		t.Errorf("expected 0 tags, got %d: %v", len(tags), tags)
	}

	tags2 := ParseOGTags("")
	if len(tags2) != 0 {
		t.Errorf("expected 0 tags for empty html, got %d", len(tags2))
	}
}

// ---------------------------------------------------------------------------
// TestPreviewCacheHitMiss — cache stores and expires entries
// ---------------------------------------------------------------------------

func TestPreviewCacheHitMiss(t *testing.T) {
	t.Parallel()

	// Use a very short TTL to test expiry.
	cache := NewPreviewCache(50 * time.Millisecond)

	url := "https://example.com/test"
	p := &URLPreview{URL: url, Title: "Cached"}

	// Miss before set.
	if _, ok := cache.Get(url); ok {
		t.Fatal("expected cache miss before Set")
	}

	cache.Set(url, p)

	// Hit after set.
	got, ok := cache.Get(url)
	if !ok {
		t.Fatal("expected cache hit after Set")
	}
	if got.Title != "Cached" {
		t.Fatalf("got title %q, want %q", got.Title, "Cached")
	}

	// Wait for expiry.
	time.Sleep(60 * time.Millisecond)
	if _, ok := cache.Get(url); ok {
		t.Fatal("expected cache miss after TTL expiry")
	}
}

// ---------------------------------------------------------------------------
// TestPreviewCacheInvalidate — manual cache invalidation
// ---------------------------------------------------------------------------

func TestPreviewCacheInvalidate(t *testing.T) {
	t.Parallel()

	cache := NewPreviewCache(5 * time.Minute)
	cache.Set("https://a.com", &URLPreview{URL: "https://a.com"})
	if cache.Size() != 1 {
		t.Fatalf("expected size 1, got %d", cache.Size())
	}
	cache.Invalidate("https://a.com")
	if cache.Size() != 0 {
		t.Fatalf("expected size 0 after invalidate, got %d", cache.Size())
	}
	if _, ok := cache.Get("https://a.com"); ok {
		t.Fatal("expected miss after invalidate")
	}
}

// ---------------------------------------------------------------------------
// TestFetchPreviewWithFakeFetcher — full fetch→parse→cache flow
// ---------------------------------------------------------------------------

func TestFetchPreviewWithFakeFetcher(t *testing.T) {
	t.Parallel()

	var fetchCount int32
	htmlBody := `<html><head>
		<meta property="og:title" content="Fetched Page">
		<meta property="og:description" content="From fake server">
		<meta property="og:image" content="https://cdn.example.com/x.png">
	</head></html>`

	ps := &PreviewService{
		cache: NewPreviewCache(5 * time.Minute),
		fetch: func(ctx context.Context, u string) (string, error) {
			atomic.AddInt32(&fetchCount, 1)
			return htmlBody, nil
		},
	}

	// First fetch — should call fetcher once.
	p, err := ps.FetchPreview("https://example.com/article")
	if err != nil {
		t.Fatalf("FetchPreview error: %v", err)
	}
	if p.Title != "Fetched Page" {
		t.Errorf("title = %q, want %q", p.Title, "Fetched Page")
	}
	if p.Description != "From fake server" {
		t.Errorf("description = %q, want %q", p.Description, "From fake server")
	}
	if p.Image != "https://cdn.example.com/x.png" {
		t.Errorf("image = %q, want %q", p.Image, "https://cdn.example.com/x.png")
	}
	if p.URL != "https://example.com/article" {
		t.Errorf("url = %q, want %q", p.URL, "https://example.com/article")
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Fatalf("expected fetch count 1, got %d", fetchCount)
	}

	// Second fetch — cache hit, fetcher NOT called again.
	p2, err := ps.FetchPreview("https://example.com/article")
	if err != nil {
		t.Fatalf("second FetchPreview error: %v", err)
	}
	if p2.Title != p.Title {
		t.Errorf("cached title mismatch: %q vs %q", p2.Title, p.Title)
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Fatalf("expected fetch count still 1 (cache hit), got %d", fetchCount)
	}
}

// ---------------------------------------------------------------------------
// TestFetchPreviewInvalidURL — invalid/empty URLs are rejected
// ---------------------------------------------------------------------------

func TestFetchPreviewInvalidURL(t *testing.T) {
	t.Parallel()

	ps := &PreviewService{
		cache: NewPreviewCache(5 * time.Minute),
		fetch: func(ctx context.Context, u string) (string, error) {
			t.Fatal("fetcher should not be called for invalid URL")
			return "", nil
		},
	}

	invalidURLs := []string{
		"",
		"not-a-url",
		"ftp://example.com",
		"javascript:alert(1)",
	}
	for _, u := range invalidURLs {
		_, err := ps.FetchPreview(u)
		if err != ErrInvalidURL {
			t.Errorf("FetchPreview(%q) err = %v, want ErrInvalidURL", u, err)
		}
	}
}

// ---------------------------------------------------------------------------
// TestFetchPreviewFetchError — fetcher failure propagates
// ---------------------------------------------------------------------------

func TestFetchPreviewFetchError(t *testing.T) {
	t.Parallel()

	ps := &PreviewService{
		cache: NewPreviewCache(5 * time.Minute),
		fetch: func(ctx context.Context, u string) (string, error) {
			return "", errors.New("network down")
		},
	}

	_, err := ps.FetchPreview("https://example.com/fail")
	if err == nil {
		t.Fatal("expected error from failed fetch")
	}
	if !strings.Contains(err.Error(), "failed to fetch") {
		t.Fatalf("expected fetch error message, got %v", err)
	}
}
