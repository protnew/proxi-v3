package mesh

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// PeerInfo describes a peer in the mesh network.
type PeerInfo struct {
	ID        string   `json:"id"`
	PublicKey string   `json:"publicKey"`
	Address   string   `json:"address"`
	Peers     []string `json:"peers"` // known peers of this peer
	LastSeen  int64    `json:"lastSeen"`
}

// MeshNet manages P2P mesh topology.
type MeshNet struct {
	mu       sync.RWMutex
	peers    map[string]*PeerInfo
	knownIDs map[string]bool // all known peer IDs in the network
	maxHops  int
}

// NewMeshNet creates a new mesh network manager.
func NewMeshNet(maxHops int) *MeshNet {
	if maxHops <= 0 {
		maxHops = 6
	}
	return &MeshNet{
		peers:    make(map[string]*PeerInfo),
		knownIDs: make(map[string]bool),
		maxHops:  maxHops,
	}
}

// AddPeer adds or updates a peer in the mesh.
func (m *MeshNet) AddPeer(info PeerInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info.LastSeen = time.Now().Unix()
	m.peers[info.ID] = &info
	m.knownIDs[info.ID] = true

	// Learn about peers this peer knows about
	for _, peerID := range info.Peers {
		m.knownIDs[peerID] = true
	}
}

// RemovePeer removes a peer from the mesh.
func (m *MeshNet) RemovePeer(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.peers, id)
	delete(m.knownIDs, id)
}

// GetPeer returns info about a specific peer.
func (m *MeshNet) GetPeer(id string) *PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.peers[id]
}

// GetAllPeers returns all known peers.
func (m *MeshNet) GetAllPeers() []PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PeerInfo, 0, len(m.peers))
	for _, p := range m.peers {
		result = append(result, *p)
	}
	return result
}

// KnownPeerCount returns total known peers (direct + indirect).
func (m *MeshNet) KnownPeerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.knownIDs)
}

// DirectPeerCount returns directly connected peers.
func (m *MeshNet) DirectPeerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.peers)
}

// FindRoute finds a route to a target peer via the mesh.
// Returns list of peer IDs forming the path, or nil if no route found.
func (m *MeshNet) FindRoute(fromID, toID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if toID == fromID {
		return nil
	}

	// BFS to find shortest path
	visited := map[string]bool{fromID: true}
	queue := [][]string{{fromID}}

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		if len(path) > m.maxHops {
			continue
		}

		current := path[len(path)-1]
		peer, ok := m.peers[current]
		if !ok {
			continue
		}

		for _, nextID := range peer.Peers {
			if nextID == toID {
				return append(path, nextID)
			}
			if !visited[nextID] {
				visited[nextID] = true
				queue = append(queue, append(append([]string{}, path...), nextID))
			}
		}
	}

	return nil // no route found
}

// GossipMessage broadcasts a message through the mesh.
// Returns the list of direct peers to forward to.
func (m *MeshNet) GossipMessage(fromID string, msg []byte, ttl int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if ttl <= 0 {
		ttl = m.maxHops
	}

	var targets []string
	for id := range m.peers {
		if id != fromID {
			targets = append(targets, id)
		}
	}
	return targets
}

// PruneStale removes peers not seen for longer than maxAge.
func (m *MeshNet) PruneStale(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()
	threshold := int64(maxAge.Seconds())

	for id, peer := range m.peers {
		if now-peer.LastSeen > threshold {
			delete(m.peers, id)
			delete(m.knownIDs, id)
			log.Printf("[mesh] pruned stale peer %s", id[:8])
		}
	}
}

// StartPruner starts a background goroutine that prunes stale peers.
func (m *MeshNet) StartPruner(interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			m.PruneStale(maxAge)
		}
	}()
}

// GetStats returns mesh network statistics.
func (m *MeshNet) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Compute average connectivity
	totalPeers := 0
	for _, p := range m.peers {
		totalPeers += len(p.Peers)
	}
	avgPeers := 0.0
	if len(m.peers) > 0 {
		avgPeers = float64(totalPeers) / float64(len(m.peers))
	}

	return map[string]interface{}{
		"directPeers": len(m.peers),
		"knownPeers":  len(m.knownIDs),
		"maxHops":     m.maxHops,
		"avgConnectivity": avgPeers,
	}
}

// MarshalPeers serializes peer list for gossip.
func (m *MeshNet) MarshalPeers() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.peers))
	for id := range m.peers {
		ids = append(ids, id)
	}
	data, _ := json.Marshal(ids)
	return data
}
