package payment

import (
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// PremiumTier defines subscription levels.
type PremiumTier string

const (
	TierFree       PremiumTier = "free"
	TierBasic      PremiumTier = "basic"
	TierPro        PremiumTier = "pro"
	TierEnterprise PremiumTier = "enterprise"
)

// FeatureLimits defines what each tier allows.
var FeatureLimits = map[PremiumTier]map[string]int{
	TierFree:       {"channels": 5, "storage_gb": 0, "api_calls_day": 100},
	TierBasic:      {"channels": 50, "storage_gb": 2, "api_calls_day": 1000},
	TierPro:        {"channels": -1, "storage_gb": 10, "api_calls_day": -1},
	TierEnterprise: {"channels": -1, "storage_gb": -1, "api_calls_day": -1},
}

// Subscription represents a user's premium subscription.
type Subscription struct {
	UserID    string      `json:"user_id"`
	Tier      PremiumTier `json:"tier"`
	ExpiresAt int64       `json:"expires_at"`
	AutoRenew bool        `json:"auto_renew"`
}

// GetSubscription returns the user's current subscription.
func GetSubscription(db *store.Store, userID string) (*Subscription, error) {
	d := db.DB()
	sub := &Subscription{}
	err := d.QueryRow(
		"SELECT user_id, tier, COALESCE(expires_at,0), COALESCE(auto_renew,0) FROM subscriptions WHERE user_id = ?",
		userID,
	).Scan(&sub.UserID, &sub.Tier, &sub.ExpiresAt, &sub.AutoRenew)
	if err != nil {
		return &Subscription{UserID: userID, Tier: TierFree}, nil
	}
	// Check expiry
	if sub.ExpiresAt > 0 && time.Now().Unix() > sub.ExpiresAt {
		return &Subscription{UserID: userID, Tier: TierFree}, nil
	}
	return sub, nil
}

// ActivatePremium sets a premium subscription for a user.
func ActivatePremium(db *store.Store, userID string, tier PremiumTier, durationDays int) error {
	d := db.DB()
	expiresAt := time.Now().Add(time.Duration(durationDays) * 24 * time.Hour).Unix()
	_, err := d.Exec(
		"INSERT OR REPLACE INTO subscriptions (user_id, tier, expires_at, auto_renew) VALUES (?, ?, ?, 1)",
		userID, tier, expiresAt,
	)
	return err
}

// CheckFeature returns whether the user's tier allows a feature at a given limit.
func CheckFeature(db *store.Store, userID string, feature string) bool {
	sub, err := GetSubscription(db, userID)
	if err != nil {
		return false
	}
	limits, ok := FeatureLimits[sub.Tier]
	if !ok {
		return false
	}
	limit, ok := limits[feature]
	if !ok {
		return false
	}
	return limit != 0 // -1 = unlimited, >0 = has limit
}

// GetTierLimits returns the limits for a given tier.
func GetTierLimits(tier PremiumTier) map[string]int {
	return FeatureLimits[tier]
}
