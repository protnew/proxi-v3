package economy

import (
	"testing"
	"time"
)

func TestRewardForStorageProof(t *testing.T) {
	ledger := NewTokenLedger()
	rep := NewReputationManager()

	minted, err := RewardForStorageProof(ledger, rep, "node-1", 0)
	if err != nil {
		t.Fatalf("reward error: %v", err)
	}
	if minted != DefaultStorageProofReward {
		t.Errorf("minted = %d, want %d", minted, DefaultStorageProofReward)
	}
	if ledger.Balance("node-1") != DefaultStorageProofReward {
		t.Errorf("balance = %d, want %d", ledger.Balance("node-1"), DefaultStorageProofReward)
	}
	rs := rep.Get("node-1")
	if rs == nil || rs.TotalProofs != 1 || rs.Score != 10 {
		t.Errorf("reputation not bumped: %+v", rs)
	}
}

func TestRewardForStorageProof_ExplicitAmount(t *testing.T) {
	ledger := NewTokenLedger()
	rep := NewReputationManager()
	minted, _ := RewardForStorageProof(ledger, rep, "node-1", 250)
	if minted != 250 {
		t.Errorf("minted = %d, want 250", minted)
	}
	if ledger.Balance("node-1") != 250 {
		t.Errorf("balance = %d, want 250", ledger.Balance("node-1"))
	}
}

func TestRewardForBandwidthProof(t *testing.T) {
	ledger := NewTokenLedger()
	rep := NewReputationManager()
	minted, err := RewardForBandwidthProof(ledger, rep, "node-1", 0)
	if err != nil {
		t.Fatalf("reward error: %v", err)
	}
	if minted != DefaultBandwidthProofReward {
		t.Errorf("minted = %d, want %d", minted, DefaultBandwidthProofReward)
	}
	if ledger.Balance("node-1") != DefaultBandwidthProofReward {
		t.Errorf("balance = %d, want %d", ledger.Balance("node-1"), DefaultBandwidthProofReward)
	}
}

func TestSlashForStorageFailure_WithStake(t *testing.T) {
	ledger := NewTokenLedger()
	rep := NewReputationManager()
	// Give the node a stake to slash.
	ledger.Mint("node-1", 1000, TokenReward)
	ledger.Stake("node-1", 500)
	rep.UpdateReputation("node-1", true) // build some reputation
	before := rep.Get("node-1").Score

	slashed, err := SlashForStorageFailure(ledger, rep, "node-1", 0)
	if err != nil {
		t.Fatalf("slash error: %v", err)
	}
	if slashed != DefaultSlashAmount {
		t.Errorf("slashed = %d, want %d", slashed, DefaultSlashAmount)
	}
	if ledger.StakedBalance("node-1") != 500-DefaultSlashAmount {
		t.Errorf("staked = %d, want %d", ledger.StakedBalance("node-1"), 500-DefaultSlashAmount)
	}
	if ledger.TotalSupply() != 1000-DefaultSlashAmount {
		t.Errorf("total supply = %d, want %d", ledger.TotalSupply(), 1000-DefaultSlashAmount)
	}
	// Reputation must drop.
	if rep.Get("node-1").Score >= before {
		t.Error("reputation should drop after slash")
	}
}

func TestSlashForStorageFailure_NoStake(t *testing.T) {
	ledger := NewTokenLedger()
	rep := NewReputationManager()
	// Node has balance but nothing staked.
	ledger.Mint("node-1", 1000, TokenReward)

	slashed, err := SlashForStorageFailure(ledger, rep, "node-1", 200)
	if err != nil {
		t.Fatalf("slash error: %v", err)
	}
	if slashed != 0 {
		t.Errorf("slashed = %d, want 0 (no stake)", slashed)
	}
	// Balance untouched when there is no stake.
	if ledger.Balance("node-1") != 1000 {
		t.Errorf("balance should be untouched, got %d", ledger.Balance("node-1"))
	}
	// But reputation is still penalized.
	if rep.Get("node-1").Score != 0 {
		t.Errorf("reputation should be floored at 0, got %d", rep.Get("node-1").Score)
	}
}

func TestSlashForStorageFailure_CappedAtStake(t *testing.T) {
	ledger := NewTokenLedger()
	rep := NewReputationManager()
	ledger.Mint("node-1", 1000, TokenReward)
	ledger.Stake("node-1", 30) // stake smaller than requested slash

	slashed, err := SlashForStorageFailure(ledger, rep, "node-1", 500)
	if err != nil {
		t.Fatalf("slash error: %v", err)
	}
	if slashed != 30 {
		t.Errorf("slashed = %d, want 30 (capped at stake)", slashed)
	}
	if ledger.StakedBalance("node-1") != 0 {
		t.Errorf("staked should be 0, got %d", ledger.StakedBalance("node-1"))
	}
}

// End-to-end: a successful storage challenge rewards the node, a timeout slashes it.
func TestIntegration_RewardThenSlash(t *testing.T) {
	ledger := NewTokenLedger()
	rep := NewReputationManager()

	// Fund + stake the node so a slash has teeth.
	ledger.Mint("node-1", 1000, TokenReward)
	ledger.Stake("node-1", 300)

	cm := NewChunkMap()
	chunkData := []byte("integration chunk")
	hash := cm.Add(chunkData)
	sched := NewStorageChallengeScheduler(cm, time.Hour, time.Hour)
	sched.SetCallbacks(SchedulerCallbacks{
		OnSuccess: func(node, chunk string) { RewardForStorageProof(ledger, rep, node, 0) },
		OnFailure: func(node, chunk, reason string) { SlashForStorageFailure(ledger, rep, node, 0) },
	})
	sched.RegisterStorage("node-1", hash)

	// Successful proof → reward.
	ch, _ := sched.IssueChallenge("node-1", hash)
	passed, err := sched.SubmitProof("node-1", hash, GenerateProofData(ch, chunkData))
	if err != nil || !passed {
		t.Fatalf("proof should pass: %v passed=%v", err, passed)
	}
	if ledger.Balance("node-1") != 800 { // 700 available + 100 reward
		t.Errorf("balance after reward = %d, want 800", ledger.Balance("node-1"))
	}

	// Failed proof → slash (300 stake - 50 slash = 250).
	ch, _ = sched.IssueChallenge("node-1", hash)
	sched.SubmitProof("node-1", hash, []byte("bad"))
	if ledger.StakedBalance("node-1") != 250 {
		t.Errorf("stake after slash = %d, want 250", ledger.StakedBalance("node-1"))
	}
}
