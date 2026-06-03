package economy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// Storage Proof Tests
// ──────────────────────────────────────────────

func TestGenerateChallenge(t *testing.T) {
	chunkData := []byte("hello world this is a chunk of data")
	chunkHash := ChunkHash(chunkData)

	ch := GenerateChallenge(chunkHash)

	if ch.ChunkHash != chunkHash {
		t.Errorf("ChunkHash mismatch: got %s, want %s", ch.ChunkHash, chunkHash)
	}
	if ch.Nonce == "" {
		t.Error("Nonce should not be empty")
	}
	if ch.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	// Two challenges for the same chunk should have different nonces
	ch2 := GenerateChallenge(chunkHash)
	if ch.Nonce == ch2.Nonce {
		t.Error("Two challenges should have different nonces")
	}
}

func TestVerifyProof_Valid(t *testing.T) {
	originalData := []byte("this is some important content stored on a node")
	chunkHash := ChunkHash(originalData)
	challenge := GenerateChallenge(chunkHash)

	proofData := GenerateProofData(challenge, originalData)

	if !VerifyProof(challenge, proofData, originalData) {
		t.Error("Valid proof should verify successfully")
	}
}

func TestVerifyProof_WrongData(t *testing.T) {
	originalData := []byte("correct data")
	chunkHash := ChunkHash(originalData)
	challenge := GenerateChallenge(chunkHash)

	proofData := GenerateProofData(challenge, originalData)

	wrongData := []byte("wrong data")
	if VerifyProof(challenge, proofData, wrongData) {
		t.Error("Proof should NOT verify with wrong data")
	}
}

func TestVerifyProof_TamperedProof(t *testing.T) {
	originalData := []byte("some data to store")
	chunkHash := ChunkHash(originalData)
	challenge := GenerateChallenge(chunkHash)

	proofData := GenerateProofData(challenge, originalData)

	// Tamper with the proof
	tampered := make([]byte, len(proofData))
	copy(tampered, proofData)
	tampered[0] ^= 0xFF

	if VerifyProof(challenge, tampered, originalData) {
		t.Error("Tampered proof should NOT verify")
	}
}

func TestVerifyProof_EmptyProof(t *testing.T) {
	originalData := []byte("some data")
	chunkHash := ChunkHash(originalData)
	challenge := GenerateChallenge(chunkHash)

	if VerifyProof(challenge, []byte{}, originalData) {
		t.Error("Empty proof should NOT verify")
	}
}

func TestChunkHash(t *testing.T) {
	data := []byte("test data")
	h1 := ChunkHash(data)
	h2 := ChunkHash(data)
	if h1 != h2 {
		t.Error("Same data should produce same chunk hash")
	}
	h3 := ChunkHash([]byte("different data"))
	if h1 == h3 {
		t.Error("Different data should produce different chunk hashes")
	}
}

// ──────────────────────────────────────────────
// Bandwidth Proof Tests
// ──────────────────────────────────────────────

func TestRecordAndVerifyTransfer(t *testing.T) {
	senderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	tr, err := RecordTransfer("node-A", "node-B", 1024000, senderKey)
	if err != nil {
		t.Fatalf("RecordTransfer failed: %v", err)
	}

	if tr.SenderID != "node-A" {
		t.Errorf("SenderID mismatch: got %s", tr.SenderID)
	}
	if tr.ReceiverID != "node-B" {
		t.Errorf("ReceiverID mismatch: got %s", tr.ReceiverID)
	}
	if tr.BytesSent != 1024000 {
		t.Errorf("BytesSent mismatch: got %d", tr.BytesSent)
	}
	if tr.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if len(tr.Signature) == 0 {
		t.Error("Signature should not be empty")
	}

	// Verify with correct public key
	if !VerifyTransfer(tr, &senderKey.PublicKey) {
		t.Error("Transfer should verify with correct public key")
	}

	// Verify with wrong public key
	wrongKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if VerifyTransfer(tr, &wrongKey.PublicKey) {
		t.Error("Transfer should NOT verify with wrong public key")
	}
}

func TestVerifyTransfer_TamperedRecord(t *testing.T) {
	senderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tr, _ := RecordTransfer("node-A", "node-B", 5000, senderKey)

	// Tamper with BytesSent
	tr.BytesSent = 99999
	if VerifyTransfer(tr, &senderKey.PublicKey) {
		t.Error("Tampered record should NOT verify")
	}
}

func TestRecordTransfer_ZeroBytes(t *testing.T) {
	senderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tr, err := RecordTransfer("sender", "receiver", 0, senderKey)
	if err != nil {
		t.Fatalf("RecordTransfer with 0 bytes failed: %v", err)
	}
	if !VerifyTransfer(tr, &senderKey.PublicKey) {
		t.Error("Zero-byte transfer should verify")
	}
}

// ──────────────────────────────────────────────
// Rewards Tests
// ──────────────────────────────────────────────

func TestCalculateReward(t *testing.T) {
	stats := NodeStats{
		NodeID:              "node-1",
		StorageProofsPassed: 100,
		BytesServed:         50000,
		UptimeHours:         72.5,
	}
	reward := CalculateReward(stats, 1000000)
	if reward <= 0 {
		t.Error("Reward should be positive for active node")
	}
}

func TestCalculateReward_ZeroPool(t *testing.T) {
	stats := NodeStats{
		NodeID:              "node-1",
		StorageProofsPassed: 100,
		BytesServed:         50000,
		UptimeHours:         72.5,
	}
	reward := CalculateReward(stats, 0)
	if reward != 0 {
		t.Errorf("Reward should be 0 for zero pool, got %d", reward)
	}
}

func TestCalculateReward_NegativePool(t *testing.T) {
	stats := NodeStats{
		NodeID:              "node-1",
		StorageProofsPassed: 100,
		BytesServed:         50000,
		UptimeHours:         72.5,
	}
	reward := CalculateReward(stats, -1000)
	if reward != 0 {
		t.Errorf("Reward should be 0 for negative pool, got %d", reward)
	}
}

func TestCalculateReward_NoActivity(t *testing.T) {
	stats := NodeStats{NodeID: "inactive-node"}
	reward := CalculateReward(stats, 1000000)
	if reward != 0 {
		t.Errorf("Inactive node should get 0 raw score, got %d", reward)
	}
}

func TestDistributeRewards_SingleNode(t *testing.T) {
	nodes := []NodeStats{
		{NodeID: "only-node", StorageProofsPassed: 50, BytesServed: 10000, UptimeHours: 24},
	}
	pool := int64(1000000)
	rewards := DistributeRewards(nodes, pool)

	if rewards["only-node"] != 1000000 {
		t.Errorf("Single node should get entire pool, got %d", rewards["only-node"])
	}
}

func TestDistributeRewards_MultipleNodes(t *testing.T) {
	nodes := []NodeStats{
		{NodeID: "node-A", StorageProofsPassed: 100, BytesServed: 50000, UptimeHours: 100},
		{NodeID: "node-B", StorageProofsPassed: 50, BytesServed: 25000, UptimeHours: 50},
	}
	pool := int64(100000)
	rewards := DistributeRewards(nodes, pool)

	// node-A should get roughly 2x node-B (scores are proportional)
	// Check total distributed equals pool
	var total int64
	for _, r := range rewards {
		total += r
	}
	if total != pool {
		t.Errorf("Total distributed (%d) should equal pool (%d)", total, pool)
	}

	if rewards["node-A"] <= rewards["node-B"] {
		t.Error("Node-A with better stats should get more reward than Node-B")
	}
}

func TestDistributeRewards_EmptyNodes(t *testing.T) {
	rewards := DistributeRewards([]NodeStats{}, 1000000)
	if len(rewards) != 0 {
		t.Error("Empty nodes should yield empty rewards map")
	}
}

func TestDistributeRewards_ZeroPool(t *testing.T) {
	nodes := []NodeStats{
		{NodeID: "node-1", StorageProofsPassed: 100},
	}
	rewards := DistributeRewards(nodes, 0)
	if len(rewards) != 0 {
		t.Error("Zero pool should yield empty rewards")
	}
}

func TestDistributeRewards_AllNodesInactive(t *testing.T) {
	nodes := []NodeStats{
		{NodeID: "node-1"},
		{NodeID: "node-2"},
	}
	rewards := DistributeRewards(nodes, 1000000)
	// All scores are 0, no distribution
	if len(rewards) != 0 {
		t.Error("All inactive nodes should yield empty rewards")
	}
}

func TestDistributeRewards_EdgeCases(t *testing.T) {
	// One active, one inactive node
	nodes := []NodeStats{
		{NodeID: "active", StorageProofsPassed: 10, BytesServed: 1000, UptimeHours: 10},
		{NodeID: "inactive"},
	}
	pool := int64(50000)
	rewards := DistributeRewards(nodes, pool)

	// Active node gets everything, inactive gets nothing
	if rewards["active"] != pool {
		t.Errorf("Single active node should get entire pool, got %d", rewards["active"])
	}
	if rewards["inactive"] != 0 {
		t.Error("Inactive node should get 0")
	}
}

// ──────────────────────────────────────────────
// Reputation Tests
// ──────────────────────────────────────────────

func TestReputationScore_UpdateReputation_Success(t *testing.T) {
	rs := &ReputationScore{NodeID: "test-node"}
	for i := 0; i < 11; i++ {
		rs.UpdateReputation(true)
	}
	if rs.Score != 110 {
		t.Errorf("Expected score 110 after 11 successes, got %d", rs.Score)
	}
	if rs.TotalProofs != 11 {
		t.Errorf("Expected 11 total proofs, got %d", rs.TotalProofs)
	}
	if rs.FailedProofs != 0 {
		t.Errorf("Expected 0 failed proofs, got %d", rs.FailedProofs)
	}
}

func TestReputationScore_UpdateReputation_Failure(t *testing.T) {
	rs := &ReputationScore{NodeID: "test-node"}
	rs.UpdateReputation(false)
	if rs.Score != 0 {
		t.Errorf("Score should floor at 0, got %d", rs.Score)
	}
	if rs.Strikes != 1 {
		t.Errorf("Expected 1 strike, got %d", rs.Strikes)
	}
	if rs.FailedProofs != 1 {
		t.Errorf("Expected 1 failed proof, got %d", rs.FailedProofs)
	}
}

func TestReputationScore_UpdateReputation_Mixed(t *testing.T) {
	rs := &ReputationScore{NodeID: "test-node"}
	// Build up score
	for i := 0; i < 20; i++ {
		rs.UpdateReputation(true)
	}
	// Score = 200
	if rs.Score != 200 {
		t.Errorf("Expected 200 after 20 successes, got %d", rs.Score)
	}
	// Now fail
	rs.UpdateReputation(false)
	// Score = 200 - 50 = 150
	if rs.Score != 150 {
		t.Errorf("Expected 150 after 1 failure, got %d", rs.Score)
	}
}

func TestReputationScore_IsTrusted(t *testing.T) {
	rs := &ReputationScore{NodeID: "test-node"}
	if rs.IsTrusted() {
		t.Error("New node should not be trusted (score 0)")
	}

	// 11 successes = 110 score
	for i := 0; i < 11; i++ {
		rs.UpdateReputation(true)
	}
	if !rs.IsTrusted() {
		t.Error("Node with score 110 should be trusted (threshold > 100)")
	}

	// Exactly 10 successes = 100 score, NOT trusted (must be > 100)
	rs2 := &ReputationScore{NodeID: "test-node-2"}
	for i := 0; i < 10; i++ {
		rs2.UpdateReputation(true)
	}
	if rs2.IsTrusted() {
		t.Error("Node with score 100 should NOT be trusted (must be > 100)")
	}
}

func TestReputationScore_StrikeReduction(t *testing.T) {
	rs := &ReputationScore{NodeID: "test-node"}
	rs.UpdateReputation(false) // strikes = 1
	rs.UpdateReputation(false) // strikes = 2
	if rs.Strikes != 2 {
		t.Errorf("Expected 2 strikes, got %d", rs.Strikes)
	}
	// Success clears one strike
	rs.UpdateReputation(true)
	if rs.Strikes != 1 {
		t.Errorf("Expected 1 strike after success, got %d", rs.Strikes)
	}
}

func TestReputationScore_ScoreFloor(t *testing.T) {
	rs := &ReputationScore{NodeID: "test-node"}
	for i := 0; i < 10; i++ {
		rs.UpdateReputation(false)
	}
	if rs.Score < 0 {
		t.Errorf("Score should never go below 0, got %d", rs.Score)
	}
	if rs.Score != 0 {
		t.Errorf("Score should be floored at 0, got %d", rs.Score)
	}
}

func TestReputationManager_GetOrCreate(t *testing.T) {
	rm := NewReputationManager()
	rs := rm.GetOrCreate("node-1")
	if rs == nil {
		t.Fatal("GetOrCreate should return a score")
	}
	if rs.NodeID != "node-1" {
		t.Errorf("NodeID mismatch: got %s", rs.NodeID)
	}

	// Second call returns the same object
	rs2 := rm.GetOrCreate("node-1")
	if rs != rs2 {
		t.Error("GetOrCreate should return same pointer for same node")
	}
}

func TestReputationManager_UpdateAndTrust(t *testing.T) {
	rm := NewReputationManager()

	// New node should not be trusted
	if rm.IsTrusted("new-node") {
		t.Error("New node should not be trusted")
	}

	// Build trust
	for i := 0; i < 15; i++ {
		rm.UpdateReputation("new-node", true)
	}
	if !rm.IsTrusted("new-node") {
		t.Error("Node with 150 score should be trusted")
	}
}

func TestReputationManager_GetTopNodes(t *testing.T) {
	rm := NewReputationManager()

	// Setup nodes with different reputations
	for i := 0; i < 20; i++ {
		rm.UpdateReputation("node-A", true) // 200
	}
	for i := 0; i < 10; i++ {
		rm.UpdateReputation("node-B", true) // 100
	}
	for i := 0; i < 5; i++ {
		rm.UpdateReputation("node-C", true) // 50
	}

	top2 := rm.GetTopNodes(2)
	if len(top2) != 2 {
		t.Fatalf("Expected 2 top nodes, got %d", len(top2))
	}
	if top2[0].NodeID != "node-A" {
		t.Errorf("Top node should be node-A, got %s", top2[0].NodeID)
	}
	if top2[1].NodeID != "node-B" {
		t.Errorf("Second node should be node-B, got %s", top2[1].NodeID)
	}

	// Request more than available
	all := rm.GetTopNodes(100)
	if len(all) != 3 {
		t.Errorf("Should return all 3 nodes when requesting 100, got %d", len(all))
	}
}

func TestReputationManager_GetTopNodes_Sorting(t *testing.T) {
	rm := NewReputationManager()

	// Add nodes in random order
	rm.UpdateReputation("low", true)    // 10
	rm.UpdateReputation("high", true)   // 10
	rm.UpdateReputation("high", true)   // 20
	rm.UpdateReputation("high", true)   // 30
	rm.UpdateReputation("mid", true)    // 10
	rm.UpdateReputation("mid", true)    // 20

	top := rm.GetTopNodes(3)
	if top[0].Score < top[1].Score {
		t.Error("Top nodes should be sorted descending by score")
	}
	if top[1].Score < top[2].Score {
		t.Error("Top nodes should be sorted descending by score")
	}
}

func TestReputationManager_Get_NonExistent(t *testing.T) {
	rm := NewReputationManager()
	rs := rm.Get("nonexistent")
	if rs != nil {
		t.Error("Get for non-existent node should return nil")
	}
}

// ──────────────────────────────────────────────
// Integration Test: Full Flow
// ──────────────────────────────────────────────

func TestIntegration_FullFlow(t *testing.T) {
	rm := NewReputationManager()

	// Simulate 3 nodes
	nodes := []string{"storage-node-1", "storage-node-2", "storage-node-3"}

	// Each node stores a chunk and responds to challenges
	chunks := map[string][]byte{
		"storage-node-1": []byte("content chunk alpha - stored on node 1"),
		"storage-node-2": []byte("content chunk beta - stored on node 2"),
		"storage-node-3": []byte("content chunk gamma - stored on node 3"),
	}

	// Simulate 5 rounds of storage proofs
	for round := 0; round < 5; round++ {
		for _, nodeID := range nodes {
			data := chunks[nodeID]
			chunkHash := ChunkHash(data)
			challenge := GenerateChallenge(chunkHash)

			if round == 3 && nodeID == "storage-node-3" {
				// Node 3 fails in round 3
				rm.UpdateReputation(nodeID, false)
				continue
			}

			proof := GenerateProofData(challenge, data)
			valid := VerifyProof(challenge, proof, data)
			if !valid {
				t.Fatalf("Round %d: valid proof failed for %s", round, nodeID)
			}
			rm.UpdateReputation(nodeID, true)
		}
	}

	// Scores:
	// node-1: 5 successes = 50
	// node-2: 5 successes = 50
	// node-3: rounds 0,1,2 = 3 successes (30), round 3 = fail (30-50=-20→0), round 4 = success (+10) = 10
	s1 := rm.Get("storage-node-1")
	s2 := rm.Get("storage-node-2")
	s3 := rm.Get("storage-node-3")

	if s1.Score != 50 {
		t.Errorf("node-1 expected score 50, got %d", s1.Score)
	}
	if s2.Score != 50 {
		t.Errorf("node-2 expected score 50, got %d", s2.Score)
	}
	// node-3: 3 success (30) + 1 fail (-50 → floored 0) + 1 success (+10) = 10
	if s3.Score != 10 {
		t.Errorf("node-3 expected score 10, got %d", s3.Score)
	}

	// Bandwidth transfers
	senderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tr, err := RecordTransfer("storage-node-1", "storage-node-2", 1_000_000, senderKey)
	if err != nil {
		t.Fatalf("RecordTransfer failed: %v", err)
	}
	if !VerifyTransfer(tr, &senderKey.PublicKey) {
		t.Error("Transfer verification failed")
	}

	// Distribute rewards
	nodeStats := []NodeStats{
		{NodeID: "storage-node-1", StorageProofsPassed: 5, BytesServed: 1_000_000, UptimeHours: 120, Reputation: s1.Score},
		{NodeID: "storage-node-2", StorageProofsPassed: 5, BytesServed: 500_000, UptimeHours: 80, Reputation: s2.Score},
		{NodeID: "storage-node-3", StorageProofsPassed: 4, BytesServed: 100_000, UptimeHours: 40, Reputation: s3.Score},
	}
	pool := int64(10_000_000)
	rewards := DistributeRewards(nodeStats, pool)

	var totalDist int64
	for _, r := range rewards {
		totalDist += r
	}
	if totalDist != pool {
		t.Errorf("Distributed total %d != pool %d", totalDist, pool)
	}

	// node-1 should get more than node-2, node-2 more than node-3
	if rewards["storage-node-1"] <= rewards["storage-node-2"] {
		t.Error("node-1 should get more reward than node-2")
	}
	if rewards["storage-node-2"] <= rewards["storage-node-3"] {
		t.Error("node-2 should get more reward than node-3")
	}

	// Top nodes
	top := rm.GetTopNodes(2)
	if len(top) != 2 {
		t.Fatalf("Expected 2 top nodes, got %d", len(top))
	}
	if top[0].Score < top[1].Score {
		t.Error("Top nodes should be sorted by score descending")
	}
}

// ──────────────────────────────────────────────
// Concurrency test for ReputationManager
// ──────────────────────────────────────────────

func TestReputationManager_Concurrent(t *testing.T) {
	rm := NewReputationManager()
	done := make(chan struct{})

	// Concurrent updates from multiple goroutines
	for g := 0; g < 10; g++ {
		go func(id string) {
			for i := 0; i < 100; i++ {
				rm.UpdateReputation(id, true)
			}
			done <- struct{}{}
		}("goroutine-" + string(rune('A'+g)))
	}

	for g := 0; g < 10; g++ {
		<-done
	}

	all := rm.AllScores()
	if len(all) != 10 {
		t.Errorf("Expected 10 nodes, got %d", len(all))
	}

	for _, rs := range all {
		// 100 successes × 10 = 1000
		if rs.Score != 1000 {
			t.Errorf("Node %s expected score 1000, got %d", rs.NodeID, rs.Score)
		}
	}
}

// Verify LastProofAt is updated
func TestReputationScore_LastProofAt(t *testing.T) {
	rs := &ReputationScore{NodeID: "test"}
	before := time.Now()
	rs.UpdateReputation(true)
	after := time.Now()

	if rs.LastProofAt.Before(before) || rs.LastProofAt.After(after) {
		t.Error("LastProofAt should be between before and after")
	}
}
