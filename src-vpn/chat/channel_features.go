package chat

import (
	"errors"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Channel Categories (folders)
// ---------------------------------------------------------------------------

// ErrCategoryNotFound is returned when a category operation references an
// unknown category ID.
var ErrCategoryNotFound = errors.New("channel category not found")

// ErrChannelNotInCategory is returned when removing a channel that is not in a
// category.
var ErrChannelNotInCategory = errors.New("channel not in category")

// ChannelCategory groups channels into a named folder (e.g. "Work", "Family").
type ChannelCategory struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	ChannelIDs []string `json:"channelIDs"`
	CreatedAt  int64    `json:"createdAt"`
}

// CategoryManager manages channel categories. It is safe for concurrent use.
type CategoryManager struct {
	mu           sync.RWMutex
	categories   map[string]*ChannelCategory
	channelToCat map[string]string // channelID → categoryID (a channel belongs to at most one category)
}

// NewCategoryManager creates a new empty CategoryManager.
func NewCategoryManager() *CategoryManager {
	return &CategoryManager{
		categories:   make(map[string]*ChannelCategory),
		channelToCat: make(map[string]string),
	}
}

// CreateCategory creates a new category. If the ID already exists the existing
// category is returned unchanged.
func (cm *CategoryManager) CreateCategory(id, name string) *ChannelCategory {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if existing, ok := cm.categories[id]; ok {
		return existing
	}
	cat := &ChannelCategory{
		ID:         id,
		Name:       name,
		ChannelIDs: []string{},
		CreatedAt:  time.Now().Unix(),
	}
	cm.categories[id] = cat
	return cat
}

// GetCategory returns the category with the given ID.
func (cm *CategoryManager) GetCategory(id string) (*ChannelCategory, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	cat, ok := cm.categories[id]
	if !ok {
		return nil, ErrCategoryNotFound
	}
	// Return a defensive copy.
	cp := *cat
	cp.ChannelIDs = append([]string(nil), cat.ChannelIDs...)
	return &cp, nil
}

// UpdateCategory renames a category.
func (cm *CategoryManager) UpdateCategory(id, name string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cat, ok := cm.categories[id]
	if !ok {
		return ErrCategoryNotFound
	}
	cat.Name = name
	return nil
}

// DeleteCategory removes a category. Channels are unassigned but not deleted.
func (cm *CategoryManager) DeleteCategory(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cat, ok := cm.categories[id]
	if !ok {
		return ErrCategoryNotFound
	}
	for _, chID := range cat.ChannelIDs {
		delete(cm.channelToCat, chID)
	}
	delete(cm.categories, id)
	return nil
}

// AddChannel assigns a channel to a category. If the channel was previously in
// another category it is moved. Returns ErrCategoryNotFound if the category
// does not exist.
func (cm *CategoryManager) AddChannel(categoryID, channelID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cat, ok := cm.categories[categoryID]
	if !ok {
		return ErrCategoryNotFound
	}

	// If channel is already in this category, no-op.
	if existingCatID, has := cm.channelToCat[channelID]; has && existingCatID == categoryID {
		return nil
	}

	// Remove from previous category if any.
	if oldCatID, has := cm.channelToCat[channelID]; has {
		cm.removeChannelFromLocked(oldCatID, channelID)
	}

	// Add to new category.
	cm.channelToCat[channelID] = categoryID
	cat.ChannelIDs = append(cat.ChannelIDs, channelID)
	return nil
}

// RemoveChannel removes a channel from its category.
func (cm *CategoryManager) RemoveChannel(channelID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	catID, has := cm.channelToCat[channelID]
	if !has {
		return ErrChannelNotInCategory
	}
	cm.removeChannelFromLocked(catID, channelID)
	delete(cm.channelToCat, channelID)
	return nil
}

// removeChannelFromLocked removes a channel from a category's channel list.
// Caller must hold cm.mu.
func (cm *CategoryManager) removeChannelFromLocked(categoryID, channelID string) {
	cat, ok := cm.categories[categoryID]
	if !ok {
		return
	}
	for i, id := range cat.ChannelIDs {
		if id == channelID {
			cat.ChannelIDs = append(cat.ChannelIDs[:i], cat.ChannelIDs[i+1:]...)
			return
		}
	}
}

// ListChannels returns the channel IDs assigned to a category.
func (cm *CategoryManager) ListChannels(categoryID string) ([]string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	cat, ok := cm.categories[categoryID]
	if !ok {
		return nil, ErrCategoryNotFound
	}
	return append([]string(nil), cat.ChannelIDs...), nil
}

// ListCategories returns all categories.
func (cm *CategoryManager) ListCategories() []*ChannelCategory {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*ChannelCategory, 0, len(cm.categories))
	for _, cat := range cm.categories {
		cp := *cat
		cp.ChannelIDs = append([]string(nil), cat.ChannelIDs...)
		result = append(result, &cp)
	}
	return result
}

// CategoryCount returns the number of categories.
func (cm *CategoryManager) CategoryCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.categories)
}

// CategoryOfChannel returns the category ID a channel belongs to, or "" if none.
func (cm *CategoryManager) CategoryOfChannel(channelID string) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.channelToCat[channelID]
}

// ---------------------------------------------------------------------------
// Subscriber Count Cache (lazy update)
// ---------------------------------------------------------------------------

// subCountEntry is a cached subscriber count with metadata for lazy refresh.
type subCountEntry struct {
	count       int64
	lastUpdated time.Time
}

// SubCountSource is a function that returns the authoritative subscriber count
// for a channel. It is called on cache misses and stale entries (lazy update).
type SubCountSource func(channelID string) int64

// SubscriberCountCache caches per-channel subscriber counts with lazy refresh.
// Entries are considered fresh for the configured TTL; on expiry or
// invalidation, the next GetCount call fetches from the source function.
type SubscriberCountCache struct {
	mu      sync.RWMutex
	entries map[string]*subCountEntry
	ttl     time.Duration
	source  SubCountSource
}

// NewSubscriberCountCache creates a cache with the given TTL and source
// function used for lazy refreshes.
func NewSubscriberCountCache(ttl time.Duration, source SubCountSource) *SubscriberCountCache {
	return &SubscriberCountCache{
		entries: make(map[string]*subCountEntry),
		ttl:     ttl,
		source:  source,
	}
}

// GetCount returns the subscriber count for a channel. If the cached value is
// fresh it is returned immediately; otherwise (missing, expired, or
// invalidated) the source function is called to refresh the cache (lazy update).
// If no source is configured, the last cached value (or 0) is returned.
func (c *SubscriberCountCache) GetCount(channelID string) int64 {
	// Fast path: read lock.
	c.mu.RLock()
	if e, ok := c.entries[channelID]; ok && c.isFreshLocked(e) {
		count := e.count
		c.mu.RUnlock()
		return count
	}
	c.mu.RUnlock()

	// Slow path: need refresh.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have
	// refreshed already).
	if e, ok := c.entries[channelID]; ok && c.isFreshLocked(e) {
		return e.count
	}

	if c.source == nil {
		if e, ok := c.entries[channelID]; ok {
			return e.count
		}
		return 0
	}

	count := c.source(channelID)
	c.entries[channelID] = &subCountEntry{
		count:       count,
		lastUpdated: time.Now(),
	}
	return count
}

// isFreshLocked reports whether an entry is within its TTL. Caller must hold
// (at least a read) lock.
func (c *SubscriberCountCache) isFreshLocked(e *subCountEntry) bool {
	return time.Since(e.lastUpdated) < c.ttl
}

// SetCount manually sets the subscriber count for a channel, marking it fresh.
func (c *SubscriberCountCache) SetCount(channelID string, count int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[channelID] = &subCountEntry{
		count:       count,
		lastUpdated: time.Now(),
	}
}

// Invalidate marks a channel's cached count as stale so that the next GetCount
// triggers a lazy refresh from the source.
func (c *SubscriberCountCache) Invalidate(channelID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[channelID]; ok {
		// Set lastUpdated far in the past so isFreshLocked returns false.
		e.lastUpdated = time.Time{}
	}
}

// InvalidateAll marks all entries as stale.
func (c *SubscriberCountCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		e.lastUpdated = time.Time{}
	}
}

// Has reports whether the cache has any entry (fresh or stale) for a channel.
func (c *SubscriberCountCache) Has(channelID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.entries[channelID]
	return ok
}

// CacheSize returns the number of entries in the cache.
func (c *SubscriberCountCache) CacheSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
