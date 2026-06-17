package economy

import (
	"testing"
)

func TestTransferTracker_RecordAndConfirm(t *testing.T) {
	tt := NewTransferTracker()

	// Both sides agree on 1MB.
	tt.RecordSent("node-A", "node-B", 1_000_000)
	tt.RecordReceived("node-B", "node-A", 1_000_000)

	confirmed, err := tt.VerifyPair("node-A", "node-B")
	if err != nil {
		t.Fatalf("expected agreement, got error: %v", err)
	}
	if confirmed != 1_000_000 {
		t.Errorf("confirmed = %d, want 1000000", confirmed)
	}
	if tt.Confirmed("node-A", "node-B") != 1_000_000 {
		t.Error("Confirmed should return verified volume")
	}
	if tt.IsFlagged("node-A", "node-B") {
		t.Error("agreed pair should not be flagged")
	}
}

func TestTransferTracker_Accumulates(t *testing.T) {
	tt := NewTransferTracker()
	tt.RecordSent("A", "B", 100)
	tt.RecordSent("A", "B", 250)
	tt.RecordReceived("B", "A", 350)

	confirmed, err := tt.VerifyPair("A", "B")
	if err != nil {
		t.Fatalf("accumulated volumes should agree: %v", err)
	}
	if confirmed != 350 {
		t.Errorf("confirmed = %d, want 350", confirmed)
	}
}

func TestTransferTracker_PeerDisagreement(t *testing.T) {
	tt := NewTransferTracker()
	// Sender inflates its number to farm rewards.
	tt.RecordSent("cheater", "victim", 5_000_000)
	tt.RecordReceived("victim", "cheater", 1_000_000)

	_, err := tt.VerifyPair("cheater", "victim")
	if err == nil {
		t.Fatal("expected disagreement error")
	}
	if !tt.IsFlagged("cheater", "victim") {
		t.Error("disagreeing pair should be flagged")
	}
	// Confirmed volume must remain zero — the inflated transfer is NOT credited.
	if tt.Confirmed("cheater", "victim") != 0 {
		t.Errorf("inflated transfer must not be confirmed, got %d", tt.Confirmed("cheater", "victim"))
	}
}

func TestTransferTracker_AntiGamingScan(t *testing.T) {
	tt := NewTransferTracker()
	// Three pairs: two honest, one gamed.
	tt.RecordSent("h1", "h2", 1000)
	tt.RecordReceived("h2", "h1", 1000)

	tt.RecordSent("h3", "h4", 500)
	tt.RecordReceived("h4", "h3", 500)

	tt.RecordSent("gamer", "target", 9999)
	tt.RecordReceived("target", "gamer", 10)

	reports := tt.AntiGamingScan()
	if len(reports) != 1 {
		t.Fatalf("expected exactly 1 gaming report, got %d", len(reports))
	}
	r := reports[0]
	if r.Sender != "gamer" || r.Receiver != "target" {
		t.Errorf("report identifies wrong pair: %+v", r)
	}
	if r.SenderReported != 9999 || r.ReceiverReported != 10 {
		t.Errorf("report volumes wrong: %+v", r)
	}
	if !tt.IsFlagged("gamer", "target") {
		t.Error("gamer pair should be flagged after scan")
	}
}

func TestTransferTracker_AntiGaming_BlocksRewardCredit(t *testing.T) {
	tt := NewTransferTracker()
	// Cheater reports 10x what was actually received.
	tt.RecordSent("cheater", "node", 1_000_000)
	tt.RecordReceived("node", "cheater", 100_000)

	if _, err := tt.VerifyPair("cheater", "node"); err == nil {
		t.Fatal("anti-gaming should block mismatched volume")
	}
	// No confirmed bytes credited to the cheater.
	if tt.Confirmed("cheater", "node") != 0 {
		t.Errorf("cheater should have 0 confirmed, got %d", tt.Confirmed("cheater", "node"))
	}
	if tt.TotalConfirmed("cheater") != 0 {
		t.Errorf("cheater total confirmed should be 0, got %d", tt.TotalConfirmed("cheater"))
	}
}

func TestTransferTracker_TotalConfirmedBothDirections(t *testing.T) {
	tt := NewTransferTracker()
	// A->B: 500, B->A: 300, both agreed.
	tt.RecordSent("A", "B", 500)
	tt.RecordReceived("B", "A", 500)
	tt.VerifyPair("A", "B")

	tt.RecordSent("B", "A", 300)
	tt.RecordReceived("A", "B", 300)
	tt.VerifyPair("B", "A")

	// A is involved in 500 (sent) + 300 (received) = 800.
	if got := tt.TotalConfirmed("A"); got != 800 {
		t.Errorf("TotalConfirmed(A) = %d, want 800", got)
	}
}

func TestTransferTracker_Reset(t *testing.T) {
	tt := NewTransferTracker()
	tt.RecordSent("A", "B", 100)
	tt.RecordReceived("B", "A", 100)
	tt.VerifyPair("A", "B")
	tt.Reset()

	if tt.Confirmed("A", "B") != 0 {
		t.Error("confirmed should be reset")
	}
	reports := tt.AntiGamingScan()
	if len(reports) != 0 {
		t.Errorf("reports should be empty after reset, got %d", len(reports))
	}
}

func TestTransferTracker_NegativeClamped(t *testing.T) {
	tt := NewTransferTracker()
	tt.RecordSent("A", "B", -50)
	tt.RecordReceived("B", "A", -50)
	confirmed, err := tt.VerifyPair("A", "B")
	if err != nil {
		t.Fatalf("clamped zeros should agree: %v", err)
	}
	if confirmed != 0 {
		t.Errorf("expected 0 confirmed, got %d", confirmed)
	}
}

func TestTransferTracker_AgreementClearsFlag(t *testing.T) {
	tt := NewTransferTracker()
	tt.RecordSent("A", "B", 100)
	tt.RecordReceived("B", "A", 50) // mismatch → flagged on scan
	tt.AntiGamingScan()
	if !tt.IsFlagged("A", "B") {
		t.Fatal("expected flag after mismatch")
	}
	// Receiver catches up, now they agree.
	tt.RecordReceived("B", "A", 50)
	if _, err := tt.VerifyPair("A", "B"); err != nil {
		t.Fatalf("should agree now: %v", err)
	}
	if tt.IsFlagged("A", "B") {
		t.Error("flag should be cleared once volumes agree")
	}
}
