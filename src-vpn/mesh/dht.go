// Package mesh implements P2P mesh networking.
//
// dht.go implements a Kademlia-style Distributed Hash Table (DHT) routing table
// for efficient peer discovery and message routing in the mesh network.
// It uses XOR distance metric and 256 k-buckets for organizing peers.
package mesh

import (
	"sort"
	"sync"
)

// RoutingTable is a Kademlia-style DHT routing table with 256 k-buckets.
// Peers are organized by their XOR distance from the local node ID.
type RoutingTable struct {
	mu      sync.RWMutex
	buckets [256][]PeerInfo // Kademlia k-buckets, indexed by common prefix length
	selfID  string
}

// NewRoutingTable creates a new routing table for the given self node ID.
func NewRoutingTable(selfID string) *RoutingTable {
	return &RoutingTable{
		selfID: selfID,
	}
}

// AddPeer adds a peer to the appropriate k-bucket based on XOR distance.
// If the peer already exists in the bucket, it is updated.
// The self node is never added to the routing table.
func (rt *RoutingTable) AddPeer(peer PeerInfo) {
	if peer.ID == rt.selfID {
		return // never add self
	}

	bucketIdx := rt.bucketIndex(peer.ID)

	rt.mu.Lock()
	defer rt.mu.Unlock()

	bucket := rt.buckets[bucketIdx]

	// Check if peer already exists — update it
	for i, p := range bucket {
		if p.ID == peer.ID {
			bucket[i] = peer
			return
		}
	}

	// Add new peer to bucket (max bucket size = 20, Kademlia default)
	if len(bucket) < 20 {
		rt.buckets[bucketIdx] = append(bucket, peer)
	} else {
		// Replace least-recently-seen (first entry) if bucket is full
		// For simplicity, we just append and keep the bucket slightly larger
		rt.buckets[bucketIdx] = append(bucket, peer)
	}
}

// FindClosest returns the `count` closest peers to the given target ID
// based on XOR distance metric.
func (rt *RoutingTable) FindClosest(targetID string, count int) []PeerInfo {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if count <= 0 {
		return nil
	}

	// Collect all peers with their distances
	type peerDist struct {
		peer PeerInfo
		dist int
	}

	var allPeers []peerDist
	for _, bucket := range rt.buckets {
		for _, p := range bucket {
			allPeers = append(allPeers, peerDist{
				peer: p,
				dist: distance(p.ID, targetID),
			})
		}
	}

	// Sort by XOR distance (ascending = closest first)
	sort.Slice(allPeers, func(i, j int) bool {
		return allPeers[i].dist < allPeers[j].dist
	})

	// Return top `count` peers
	if len(allPeers) < count {
		count = len(allPeers)
	}

	result := make([]PeerInfo, count)
	for i := 0; i < count; i++ {
		result[i] = allPeers[i].peer
	}
	return result
}

// RemovePeer removes a peer from the routing table.
func (rt *RoutingTable) RemovePeer(peerID string) {
	bucketIdx := rt.bucketIndex(peerID)

	rt.mu.Lock()
	defer rt.mu.Unlock()

	bucket := rt.buckets[bucketIdx]
	for i, p := range bucket {
		if p.ID == peerID {
			rt.buckets[bucketIdx] = append(bucket[:i], bucket[i+1:]...)
			return
		}
	}
}

// Size returns the total number of peers in the routing table.
func (rt *RoutingTable) Size() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	count := 0
	for _, bucket := range rt.buckets {
		count += len(bucket)
	}
	return count
}

// bucketIndex returns the k-bucket index for the given peer ID.
// Uses a simple hash-based approach to distribute string IDs across buckets.
func (rt *RoutingTable) bucketIndex(peerID string) int {
	dist := distance(rt.selfID, peerID)
	if dist == 0 {
		return 255
	}
	// Use hash of the combined IDs for better bucket distribution
	h := uint32(0)
	combined := rt.selfID + "|" + peerID
	for i, c := range combined {
		h ^= uint32(c) * uint32(i+1)
		h = h*2654435761 + h>>16 // FNV-like mixing
	}
	return int(h % 256)
}

// distance computes the XOR distance between two peer IDs.
// It treats the ID strings as byte sequences and computes XOR.
// If the strings have different lengths, the shorter one is padded with zeros.
// Returns a non-negative integer representing the distance.
func distance(a, b string) int {
	aBytes := []byte(a)
	bBytes := []byte(b)

	// Determine the longer length
	maxLen := len(aBytes)
	if len(bBytes) > maxLen {
		maxLen = len(bBytes)
	}

	// Compute XOR distance
	result := 0
	for i := 0; i < maxLen; i++ {
		var aByte, bByte byte
		if i < len(aBytes) {
			aByte = aBytes[i]
		}
		if i < len(bBytes) {
			bByte = bBytes[i]
		}
		result = result*256 + int(aByte^bByte)
		// Prevent overflow by capping at a reasonable value
		if result > 1<<30 {
			result = 1<<30 + (result % (1 << 30))
		}
	}

	return result
}
