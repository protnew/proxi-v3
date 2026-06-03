package economy

import (
	"sort"
	"sync"
	"time"
)

// ReputationScore tracks a single node's reputation.
type ReputationScore struct {
	NodeID      string
	Score       int64
	LastProofAt time.Time
	TotalProofs int64
	FailedProofs int64
	Strikes     int
}

// UpdateReputation adjusts the reputation based on proof result.
// +10 for successful proof, -50 for failed proof.
func (rs *ReputationScore) UpdateReputation(proofPassed bool) {
	rs.TotalProofs++
	rs.LastProofAt = time.Now()
	if proofPassed {
		rs.Score += 10
		// Clear a strike on success
		if rs.Strikes > 0 {
			rs.Strikes--
		}
	} else {
		rs.Score -= 50
		rs.FailedProofs++
		rs.Strikes++
	}
	// Score floor at 0
	if rs.Score < 0 {
		rs.Score = 0
	}
}

// IsTrusted returns true if the node's score exceeds the trust threshold of 100.
func (rs *ReputationScore) IsTrusted() bool {
	return rs.Score > 100
}

// ReputationManager manages reputation scores for all nodes in memory.
type ReputationManager struct {
	mu    sync.RWMutex
	scores map[string]*ReputationScore
}

// NewReputationManager creates a new ReputationManager.
func NewReputationManager() *ReputationManager {
	return &ReputationManager{
		scores: make(map[string]*ReputationScore),
	}
}

// GetOrCreate returns the ReputationScore for a node, creating one if needed.
func (rm *ReputationManager) GetOrCreate(nodeID string) *ReputationScore {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rs, ok := rm.scores[nodeID]; ok {
		return rs
	}
	rs := &ReputationScore{NodeID: nodeID}
	rm.scores[nodeID] = rs
	return rs
}

// Get returns the ReputationScore for a node, or nil if not found.
func (rm *ReputationManager) Get(nodeID string) *ReputationScore {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.scores[nodeID]
}

// UpdateReputation updates the reputation for a given node.
func (rm *ReputationManager) UpdateReputation(nodeID string, proofPassed bool) {
	rs := rm.GetOrCreate(nodeID)
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rs.UpdateReputation(proofPassed)
}

// IsTrusted checks if a node is trusted.
func (rm *ReputationManager) IsTrusted(nodeID string) bool {
	rs := rm.Get(nodeID)
	if rs == nil {
		return false
	}
	return rs.IsTrusted()
}

// GetTopNodes returns the top N nodes sorted by reputation score descending.
func (rm *ReputationManager) GetTopNodes(n int) []*ReputationScore {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	all := make([]*ReputationScore, 0, len(rm.scores))
	for _, rs := range rm.scores {
		all = append(all, rs)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Score > all[j].Score
	})
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// AllScores returns a copy of all reputation scores.
func (rm *ReputationManager) AllScores() []*ReputationScore {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	all := make([]*ReputationScore, 0, len(rm.scores))
	for _, rs := range rm.scores {
		all = append(all, rs)
	}
	return all
}
