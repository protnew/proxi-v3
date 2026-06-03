package economy

import "sort"

// NodeStats holds statistics for a node used in reward calculation.
type NodeStats struct {
	NodeID               string
	StorageProofsPassed  int64
	StorageProofsFailed  int64
	BytesServed          int64
	UptimeHours          float64
	Reputation           int64
}

// CalculateReward computes the reward for a single node based on its stats
// and the total reward pool.
//
// Formula:
//   - 60% weighted by storage proofs passed
//   - 30% weighted by bytes served (bandwidth)
//   - 10% weighted by uptime hours
//
// Returns the individual reward in the same units as poolTotal.
func CalculateReward(stats NodeStats, poolTotal int64) int64 {
	if poolTotal <= 0 {
		return 0
	}
	// The function computes a score for this node; actual distribution
	// is done by DistributeRewards which normalizes across all nodes.
	// Here we just return the raw weighted score.
	storageScore := float64(stats.StorageProofsPassed) * 0.6
	bandwidthScore := float64(stats.BytesServed) * 0.3
	uptimeScore := stats.UptimeHours * 0.1
	total := storageScore + bandwidthScore + uptimeScore
	return int64(total)
}

// calculateRewardShare computes a node's share of the pool given its score
// and the sum of all scores.
func calculateRewardShare(nodeScore, totalScore, poolTotal int64) int64 {
	if totalScore <= 0 || nodeScore <= 0 {
		return 0
	}
	return (nodeScore * poolTotal) / totalScore
}

// DistributeRewards distributes poolTotal among nodes proportionally
// to their weighted scores.
// Returns a map of NodeID → reward amount.
func DistributeRewards(nodeStats []NodeStats, poolTotal int64) map[string]int64 {
	rewards := make(map[string]int64)
	if poolTotal <= 0 || len(nodeStats) == 0 {
		return rewards
	}

	// Compute raw scores
	type nodeScore struct {
		id    string
		score int64
	}
	scores := make([]nodeScore, 0, len(nodeStats))
	var totalScore int64
	for _, ns := range nodeStats {
		s := CalculateReward(ns, poolTotal)
		totalScore += s
		scores = append(scores, nodeScore{id: ns.NodeID, score: s})
	}

	if totalScore == 0 {
		return rewards
	}

	// Distribute proportionally
	var distributed int64
	for i, ns := range scores {
		var r int64
		if i == len(scores)-1 {
			// Last node gets the remainder to avoid dust loss
			r = poolTotal - distributed
		} else {
			r = calculateRewardShare(ns.score, totalScore, poolTotal)
			distributed += r
		}
		if r < 0 {
			r = 0
		}
		rewards[ns.id] = r
	}

	return rewards
}

// SortNodesByReputation returns nodes sorted by reputation descending.
func SortNodesByReputation(nodes []NodeStats) []NodeStats {
	sorted := make([]NodeStats, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Reputation > sorted[j].Reputation
	})
	return sorted
}
