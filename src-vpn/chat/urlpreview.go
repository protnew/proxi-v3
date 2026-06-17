package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DefaultPreviewTTL is the cache lifetime for fetched URL previews (5 minutes).
const DefaultPreviewTTL = 5 * time.Minute

// previewMaxBody limits how much HTML we read when fetching a preview (1 MB).
const previewMaxBody = 1 << 20

// URLPreview holds the parsed OpenGraph metadata for a URL.
type URLPreview struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
}

// ErrInvalidURL is returned when the provided URL is empty or malformed.
var ErrInvalidURL = errors.New("invalid url")

// ErrPreviewFetch is returned when the HTTP fetch for a preview fails.
var ErrPreviewFetch = errors.New("failed to fetch url preview")

// cacheEntry stores a preview with its expiration time.
type cacheEntry struct {
	preview  *URLPreview
	expiresAt time.Time
}

// PreviewCache is a TTL-based cache for URL previews. Safe for concurrent use.
type PreviewCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

// NewPreviewCache creates a cache with the given TTL.
func NewPreviewCache(ttl time.Duration) *PreviewCache {
	return &PreviewCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// Get returns a cached preview if present and not expired.
func (c *PreviewCache) Get(url string) (*URLPreview, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[url]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.preview, true
}

// Set stores a preview in the cache with the configured TTL.
func (c *PreviewCache) Set(url string, p *URLPreview) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[url] = &cacheEntry{
		preview:  p,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Invalidate removes a specific URL from the cache.
func (c *PreviewCache) Invalidate(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, url)
}

// Size returns the number of entries currently in the cache (including expired
// ones that have not been evicted).
func (c *PreviewCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// fetchFunc is the signature of a function that fetches raw HTML for a URL.
// It is abstracted so tests can inject a fake fetcher without real network.
type fetchFunc func(ctx context.Context, url string) (string, error)

// PreviewService fetches and caches URL previews (OpenGraph metadata).
type PreviewService struct {
	cache   *PreviewCache
	fetch   fetchFunc
	client  *http.Client
}

// NewPreviewService creates a PreviewService with the default HTTP client and
// a cache using the given TTL.
func NewPreviewService(ttl time.Duration) *PreviewService {
	return &PreviewService{
		cache: NewPreviewCache(ttl),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchPreview fetches a URL, parses its OpenGraph tags, and returns a
// URLPreview. Results are cached for the configured TTL. The cache is checked
// first; on a miss the URL is fetched.
func (ps *PreviewService) FetchPreview(url string) (*URLPreview, error) {
	if !isValidURL(url) {
		return nil, ErrInvalidURL
	}

	// Cache hit?
	if p, ok := ps.cache.Get(url); ok {
		return p, nil
	}

	// Fetch.
	var fetcher fetchFunc
	if ps.fetch != nil {
		fetcher = ps.fetch
	} else {
		fetcher = ps.httpFetch
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	html, err := fetcher(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPreviewFetch, err)
	}

	tags := ParseOGTags(html)
	preview := &URLPreview{
		URL:         url,
		Title:       tags["title"],
		Description: tags["description"],
		Image:       tags["image"],
	}
	ps.cache.Set(url, preview)
	return preview, nil
}

// httpFetch performs a real HTTP GET and returns the response body as a string
// (truncated to previewMaxBody).
func (ps *PreviewService) httpFetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "ProxiMessenger/1.0 (URL Preview Bot)")
	resp, err := ps.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, previewMaxBody))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// isValidURL performs a minimal validation: non-empty and has an http(s) scheme.
func isValidURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	lower := strings.ToLower(rawURL)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return len(rawURL) > 7
	}
	return false
}

// metaTagRE matches <meta ...> tags. We then inspect attributes individually.
var metaTagRE = regexp.MustCompile(`(?i)<meta\b[^>]*/?>`)

// ogPropRE extracts the og:* property value from a meta tag's attributes.
var ogPropRE = regexp.MustCompile(`(?i)property\s*=\s*["']\s*og:([a-zA-Z0-9_:.-]+)\s*["']`)

// ogContentRE extracts the content attribute value from a meta tag.
var ogContentRE = regexp.MustCompile(`(?i)\bcontent\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// ParseOGTags extracts OpenGraph meta tags from an HTML document and returns a
// map keyed by the og: property name (without the "og:" prefix). When a
// property appears multiple times the first occurrence wins.
func ParseOGTags(html string) map[string]string {
	result := make(map[string]string)
	tags := metaTagRE.FindAllString(html, -1)
	for _, tag := range tags {
		propMatch := ogPropRE.FindStringSubmatch(tag)
		if len(propMatch) < 2 {
			continue
		}
		prop := strings.TrimSpace(propMatch[1])
		if prop == "" {
			continue
		}
		if _, exists := result[prop]; exists {
			continue // first occurrence wins
		}
		contentMatch := ogContentRE.FindStringSubmatch(tag)
		var content string
		if len(contentMatch) >= 2 && contentMatch[1] != "" {
			content = contentMatch[1]
		} else if len(contentMatch) >= 3 && contentMatch[2] != "" {
			content = contentMatch[2]
		}
		if content != "" {
			result[prop] = decodeHTMLEntities(content)
		}
	}
	return result
}

// decodeHTMLEntities unescapes the most common HTML entities found in og:tags.
func decodeHTMLEntities(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&#x27;", "'",
		"&apos;", "'",
	)
	return r.Replace(s)
}
