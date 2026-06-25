package mesh

import (
	"log"

	lru "github.com/hashicorp/golang-lru/v2"
)

// PeerCache is a thread-safe LRU cache for Peer records.
// It helps to reduce memory usage and avoid keeping all peers in memory simultaneously,
// while providing fast O(1) access to active peers.
type PeerCache struct {
	cache *lru.Cache[string, *PeerInfo]
}

// NewPeerCache initializes a new PeerCache with the given size.
func NewPeerCache(size int) *PeerCache {
	c, err := lru.New[string, *PeerInfo](size)
	if err != nil {
		log.Fatalf("failed to create lru cache: %v", err)
	}
	return &PeerCache{
		cache: c,
	}
}

// Add adds or updates a peer in the cache.
func (c *PeerCache) Add(p *PeerInfo) {
	c.cache.Add(p.ID, p)
}

// Get retrieves a peer from the cache by ID.
func (c *PeerCache) Get(id string) (*PeerInfo, bool) {
	return c.cache.Get(id)
}

// Remove removes a peer from the cache.
func (c *PeerCache) Remove(id string) {
	c.cache.Remove(id)
}

// Len returns the number of items in the cache.
func (c *PeerCache) Len() int {
	return c.cache.Len()
}

// Keys returns all peer IDs in the cache.
func (c *PeerCache) Keys() []string {
	return c.cache.Keys()
}

// MessageCache defines an interface for caching seen message IDs
// to prevent infinite loops in the P2P mesh network.
type MessageCache interface {
	Add(msgID string) bool
	Contains(msgID string) bool
}

type LRUMessageCache struct {
	cache *lru.Cache[string, struct{}]
}

// NewLRUMessageCache creates a new LRU cache for message IDs with the specified size.
func NewLRUMessageCache(size int) (*LRUMessageCache, error) {
	c, err := lru.New[string, struct{}](size)
	if err != nil {
		return nil, err
	}
	return &LRUMessageCache{cache: c}, nil
}

func (c *LRUMessageCache) Add(msgID string) bool {
	ok, _ := c.cache.ContainsOrAdd(msgID, struct{}{})
	return !ok // if it wasn't there (ContainsOrAdd returns true if it WAS there)
}

func (c *LRUMessageCache) Contains(msgID string) bool {
	return c.cache.Contains(msgID)
}
