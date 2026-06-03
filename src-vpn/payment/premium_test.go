package payment

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGetSubscription_ActivePro(t *testing.T) {
	db := newTestDB(t)
	ActivatePremium(db, "user1", TierPro, 30)
	sub, err := GetSubscription(db, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if sub.Tier != TierPro {
		t.Errorf("expected pro, got %s", sub.Tier)
	}
	if sub.UserID != "user1" {
		t.Errorf("user_id mismatch: %s", sub.UserID)
	}
	if !sub.AutoRenew {
		t.Error("auto_renew should be true")
	}
}

func TestGetSubscription_Expired(t *testing.T) {
	db := newTestDB(t)
	// Insert expired subscription directly
	d := db.DB()
	expiresAt := time.Now().Add(-24 * time.Hour).Unix()
	_, err := d.Exec(
		"INSERT OR REPLACE INTO subscriptions (user_id, tier, expires_at, auto_renew) VALUES (?, ?, ?, 1)",
		"expired_user", "pro", expiresAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := GetSubscription(db, "expired_user")
	if err != nil {
		t.Fatal(err)
	}
	if sub.Tier != TierFree {
		t.Errorf("expired subscription should downgrade to free, got %s", sub.Tier)
	}
}

func TestGetSubscription_NonExistentUser(t *testing.T) {
	db := newTestDB(t)
	sub, err := GetSubscription(db, "ghost")
	if err != nil {
		t.Fatal(err)
	}
	if sub.Tier != TierFree {
		t.Errorf("non-existent user should be free, got %s", sub.Tier)
	}
	if sub.UserID != "ghost" {
		t.Errorf("user_id should be ghost, got %s", sub.UserID)
	}
}

func TestActivatePremium_Upgrade(t *testing.T) {
	db := newTestDB(t)
	err := ActivatePremium(db, "alice", TierBasic, 7)
	if err != nil {
		t.Fatal(err)
	}
	sub, _ := GetSubscription(db, "alice")
	if sub.Tier != TierBasic {
		t.Errorf("expected basic, got %s", sub.Tier)
	}
	// Upgrade to enterprise
	err = ActivatePremium(db, "alice", TierEnterprise, 365)
	if err != nil {
		t.Fatal(err)
	}
	sub, _ = GetSubscription(db, "alice")
	if sub.Tier != TierEnterprise {
		t.Errorf("expected enterprise after upgrade, got %s", sub.Tier)
	}
}

func TestCheckFeature_FreeTier(t *testing.T) {
	db := newTestDB(t)
	// Free user — channels has limit 5, so feature should be allowed (non-zero)
	if !CheckFeature(db, "freeuser", "channels") {
		t.Error("free tier should have channels feature (limit=5)")
	}
	// storage_gb is 0 for free tier — should return false
	if CheckFeature(db, "freeuser", "storage_gb") {
		t.Error("free tier should NOT have storage feature (limit=0)")
	}
}

func TestCheckFeature_UnknownFeature(t *testing.T) {
	db := newTestDB(t)
	ActivatePremium(db, "alice", TierPro, 30)
	if CheckFeature(db, "alice", "nonexistent_feature") {
		t.Error("unknown feature should return false")
	}
}

func TestCheckFeature_UnknownTier(t *testing.T) {
	db := newTestDB(t)
	d := db.DB()
	// Insert a subscription with an unknown tier
	d.Exec(
		"INSERT OR REPLACE INTO subscriptions (user_id, tier, expires_at, auto_renew) VALUES (?, ?, ?, 1)",
		"hackerman", "platinum", time.Now().Add(24*time.Hour).Unix(),
	)
	if CheckFeature(db, "hackerman", "channels") {
		t.Error("unknown tier should not have features")
	}
}

func TestGetTierLimits_AllTiers(t *testing.T) {
	for _, tier := range []PremiumTier{TierFree, TierBasic, TierPro, TierEnterprise} {
		limits := GetTierLimits(tier)
		if limits == nil {
			t.Errorf("tier %s should have limits", tier)
		}
		if _, ok := limits["channels"]; !ok {
			t.Errorf("tier %s should have channels limit", tier)
		}
	}
}

func TestGetTierLimits_UnknownTier(t *testing.T) {
	limits := GetTierLimits("unknown")
	if limits != nil {
		t.Error("unknown tier should return nil limits")
	}
}

func TestPremiumTier_Values(t *testing.T) {
	if TierFree != "free" {
		t.Errorf("TierFree should be 'free', got '%s'", TierFree)
	}
	if TierBasic != "basic" {
		t.Errorf("TierBasic should be 'basic', got '%s'", TierBasic)
	}
	if TierPro != "pro" {
		t.Errorf("TierPro should be 'pro', got '%s'", TierPro)
	}
	if TierEnterprise != "enterprise" {
		t.Errorf("TierEnterprise should be 'enterprise', got '%s'", TierEnterprise)
	}
}

func TestFeatureLimits_Values(t *testing.T) {
	entLimits := FeatureLimits[TierEnterprise]
	if entLimits["channels"] != -1 {
		t.Error("enterprise should have unlimited channels")
	}
	if entLimits["storage_gb"] != -1 {
		t.Error("enterprise should have unlimited storage")
	}
	if entLimits["api_calls_day"] != -1 {
		t.Error("enterprise should have unlimited api_calls_day")
	}
	basicLimits := FeatureLimits[TierBasic]
	if basicLimits["channels"] != 50 {
		t.Errorf("basic should have 50 channels, got %d", basicLimits["channels"])
	}
	if basicLimits["storage_gb"] != 2 {
		t.Errorf("basic should have 2gb storage, got %d", basicLimits["storage_gb"])
	}
	if basicLimits["api_calls_day"] != 1000 {
		t.Errorf("basic should have 1000 api_calls_day, got %d", basicLimits["api_calls_day"])
	}
}

func TestCheckFeature_EnterpriseUnlimited(t *testing.T) {
	db := newTestDB(t)
	ActivatePremium(db, "bigcorp", TierEnterprise, 365)
	for _, feat := range []string{"channels", "storage_gb", "api_calls_day"} {
		if !CheckFeature(db, "bigcorp", feat) {
			t.Errorf("enterprise should have %s feature", feat)
		}
	}
}

func TestSubscription_JSON(t *testing.T) {
	sub := &Subscription{
		UserID:    "u1",
		Tier:      TierPro,
		ExpiresAt: 1700000000,
		AutoRenew: true,
	}
	data, err := json.Marshal(sub)
	if err != nil {
		t.Fatal(err)
	}
	var sub2 Subscription
	if err := json.Unmarshal(data, &sub2); err != nil {
		t.Fatal(err)
	}
	if sub2.UserID != "u1" || sub2.Tier != TierPro || sub2.ExpiresAt != 1700000000 || !sub2.AutoRenew {
		t.Errorf("JSON roundtrip failed: %+v", sub2)
	}
}
