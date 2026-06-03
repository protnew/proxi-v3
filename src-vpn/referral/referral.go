package referral

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// Referral represents a referral code with usage tracking.
type Referral struct {
	Code       string `json:"code"`
	ReferrerID string `json:"referrerId"`
	Uses       int    `json:"uses"`
	MaxUses    int    `json:"maxUses"`
	CreatedAt  int64  `json:"createdAt"`
}

// generateCode creates a random 8-character referral code.
func generateCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateReferralCode generates a new referral code for the given user.
func CreateReferralCode(db *store.Store, userID string) (string, error) {
	code, err := generateCode()
	if err != nil {
		return "", err
	}
	_, err = db.DB().Exec(
		`INSERT INTO referrals (code, referrer_id, uses, max_uses, created_at) VALUES (?, ?, 0, 100, ?)`,
		code, userID, time.Now().Unix(),
	)
	if err != nil {
		return "", fmt.Errorf("create referral: %w", err)
	}
	return code, nil
}

// ApplyReferral applies a referral code for a new user. Returns the bonus days (30).
// Returns an error if the code is invalid or has reached max uses.
func ApplyReferral(db *store.Store, code, newUserID string) (bonusDays int, err error) {
	// Look up the referral code
	var referrerID string
	var uses, maxUses int
	err = db.DB().QueryRow(
		`SELECT referrer_id, uses, max_uses FROM referrals WHERE code = ?`, code,
	).Scan(&referrerID, &uses, &maxUses)
	if err != nil {
		return 0, fmt.Errorf("referral code %s not found: %w", code, err)
	}

	if uses >= maxUses {
		return 0, fmt.Errorf("referral code %s has reached maximum uses", code)
	}

	// Increment uses count
	_, err = db.DB().Exec(
		`UPDATE referrals SET uses = uses + 1 WHERE code = ?`, code,
	)
	if err != nil {
		return 0, fmt.Errorf("update referral uses: %w", err)
	}

	// Record the referral use
	_, err = db.DB().Exec(
		`INSERT INTO referral_uses (code, new_user_id, bonus_days, used_at) VALUES (?, ?, 30, ?)`,
		code, newUserID, time.Now().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("record referral use: %w", err)
	}

	return 30, nil
}

// GetReferralStats returns statistics about referrals for a given user.
func GetReferralStats(db *store.Store, userID string) (map[string]interface{}, error) {
	// Get all referral codes for this user
	rows, err := db.DB().Query(
		`SELECT code, uses, max_uses, created_at FROM referrals WHERE referrer_id = ?`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get referral codes: %w", err)
	}
	defer rows.Close()

	type codeInfo struct {
		Code     string `json:"code"`
		Uses     int    `json:"uses"`
		MaxUses  int    `json:"maxUses"`
		CreateAt int64  `json:"createdAt"`
	}

	var codes []codeInfo
	totalUses := 0
	for rows.Next() {
		var ci codeInfo
		if err := rows.Scan(&ci.Code, &ci.Uses, &ci.MaxUses, &ci.CreateAt); err != nil {
			return nil, fmt.Errorf("scan referral: %w", err)
		}
		codes = append(codes, ci)
		totalUses += ci.Uses
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Get total referred users
	var totalReferred int
	_ = db.DB().QueryRow(
		`SELECT COUNT(*) FROM referral_uses ru
		 JOIN referrals r ON ru.code = r.code
		 WHERE r.referrer_id = ?`, userID,
	).Scan(&totalReferred)

	return map[string]interface{}{
		"codes":         codes,
		"totalUses":     totalUses,
		"totalReferred": totalReferred,
	}, nil
}
