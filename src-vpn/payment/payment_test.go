package payment

import (
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

func newTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLightningClient_IsAvailable(t *testing.T) {
	client := NewLightningClient("http://localhost:19999", "", "")
	if client.IsAvailable() {
		t.Error("should not be available without LND")
	}
}

func TestGetSubscription_Free(t *testing.T) {
	db := newTestDB(t)
	sub, err := GetSubscription(db, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if sub.Tier != TierFree {
		t.Errorf("expected free tier, got %s", sub.Tier)
	}
}

func TestActivatePremium(t *testing.T) {
	db := newTestDB(t)
	err := ActivatePremium(db, "alice", TierPro, 30)
	if err != nil {
		t.Fatal(err)
	}
	sub, _ := GetSubscription(db, "alice")
	if sub.Tier != TierPro {
		t.Errorf("expected pro, got %s", sub.Tier)
	}
	if sub.ExpiresAt == 0 {
		t.Error("expires_at should be set")
	}
}

func TestCheckFeature(t *testing.T) {
	db := newTestDB(t)
	ActivatePremium(db, "alice", TierPro, 30)
	if !CheckFeature(db, "alice", "channels") {
		t.Error("pro should have channels feature")
	}
	if !CheckFeature(db, "alice", "storage_gb") {
		t.Error("pro should have storage")
	}
}

func TestGetTierLimits(t *testing.T) {
	freeLimits := GetTierLimits(TierFree)
	if freeLimits["channels"] != 5 {
		t.Errorf("free should have 5 channels, got %d", freeLimits["channels"])
	}
	proLimits := GetTierLimits(TierPro)
	if proLimits["channels"] != -1 {
		t.Errorf("pro should have unlimited channels, got %d", proLimits["channels"])
	}
}

func TestDonations(t *testing.T) {
	db := newTestDB(t)
	CreateDonation(db, "alice", "ch1", 1000)
	CreateDonation(db, "bob", "ch1", 500)

	donations, err := GetDonations(db, "ch1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(donations) != 2 {
		t.Fatalf("expected 2 donations, got %d", len(donations))
	}
	total, _ := GetTotalDonations(db, "ch1")
	if total != 1500 {
		t.Errorf("expected 1500 total sats, got %d", total)
	}
}
