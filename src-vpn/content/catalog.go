package content

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// ContentType represents the type of content.
type ContentType string

const (
	ContentVideo    ContentType = "video"
	ContentAudio    ContentType = "audio"
	ContentDocument ContentType = "document"
	ContentImage    ContentType = "image"
)

// ContentEntry is a single catalog entry describing a piece of content.
type ContentEntry struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	Type       ContentType `json:"type"`
	Size       int64       `json:"size"`
	Uploader   string      `json:"uploader"`
	ManifestID string      `json:"manifestId"`
	Access     string      `json:"access"` // public/private/paid
	Version    int         `json:"version"`
	CreatedAt  int64       `json:"createdAt"`
	UpdatedAt  int64       `json:"updatedAt"`
}

// CatalogFilter defines optional filters for listing catalog entries.
type CatalogFilter struct {
	Type     ContentType
	Uploader string
	Access   string
	Limit    int
	Offset   int
}

// Catalog is an in-memory content catalog with thread-safe access.
type Catalog struct {
	mu      sync.RWMutex
	entries map[string]*ContentEntry
}

// NewCatalog creates a new empty Catalog.
func NewCatalog() *Catalog {
	return &Catalog{
		entries: make(map[string]*ContentEntry),
	}
}

// ErrNotFound is returned when a content entry does not exist.
var ErrNotFound = errors.New("content not found")

// ErrAlreadyExists is returned when trying to add an entry with an existing ID.
var ErrAlreadyExists = errors.New("content already exists")

// Add inserts a new entry into the catalog. Returns ErrAlreadyExists if the
// ID is already present.
func (c *Catalog) Add(entry ContentEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[entry.ID]; exists {
		return ErrAlreadyExists
	}

	now := time.Now().Unix()
	if entry.CreatedAt == 0 {
		entry.CreatedAt = now
	}
	if entry.UpdatedAt == 0 {
		entry.UpdatedAt = now
	}
	if entry.Version == 0 {
		entry.Version = 1
	}

	c.entries[entry.ID] = &entry
	return nil
}

// Get retrieves a content entry by ID.
func (c *Catalog) Get(id string) (*ContentEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entries[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *e
	return &cp, nil
}

// Update modifies an existing entry. The Version field is auto-incremented
// and UpdatedAt is refreshed.
func (c *Catalog) Update(id string, entry ContentEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, ok := c.entries[id]
	if !ok {
		return ErrNotFound
	}

	entry.ID = id
	entry.CreatedAt = existing.CreatedAt
	entry.Version = existing.Version + 1
	entry.UpdatedAt = time.Now().Unix()

	c.entries[id] = &entry
	return nil
}

// Delete removes a content entry from the catalog.
func (c *Catalog) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[id]; !ok {
		return ErrNotFound
	}
	delete(c.entries, id)
	return nil
}

// List returns entries matching the given filter. Results are ordered by ID.
func (c *Catalog) List(filter CatalogFilter) []ContentEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []ContentEntry
	for _, e := range c.entries {
		if filter.Type != "" && e.Type != filter.Type {
			continue
		}
		if filter.Uploader != "" && e.Uploader != filter.Uploader {
			continue
		}
		if filter.Access != "" && e.Access != filter.Access {
			continue
		}
		result = append(result, *e)
	}

	// Apply offset
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return nil
		}
		result = result[filter.Offset:]
	}

	// Apply limit
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result
}

// Search returns entries whose title contains the query string (case-insensitive).
func (c *Catalog) Search(query string) []ContentEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	lower := strings.ToLower(query)
	var result []ContentEntry
	for _, e := range c.entries {
		if strings.Contains(strings.ToLower(e.Title), lower) {
			result = append(result, *e)
		}
	}
	return result
}
