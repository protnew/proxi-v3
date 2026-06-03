package referral

import (
	"path/filepath"
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

func newTestDB(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateReferralCode(t *testing.T) {
	db := newTestDB(t)

	code, err := CreateReferralCode(db, "user1")
	if err != nil {
		t.Fatalf("CreateReferralCode: %v", err)
	}
	if len(code) != 8 {
		t.Errorf("code length = %d, want 8", len(code))
	}

	// Create a second code for the same user — should succeed
	code2, err := CreateReferralCode(db, "user1")
	if err != nil {
		t.Fatalf("CreateReferralCode second: %v", err)
	}
	if code2 == code {
		t.Error("second code should differ from first")
	}
}

func TestApplyReferral(t *testing.T) {
	db := newTestDB(t)

	code, err := CreateReferralCode(db, "referrer1")
	if err != nil {
		t.Fatalf("CreateReferralCode: %v", err)
	}

	bonus, err := ApplyReferral(db, code, "newuser1")
	if err != nil {
		t.Fatalf("ApplyReferral: %v", err)
	}
	if bonus != 30 {
		t.Errorf("bonus = %d, want 30", bonus)
	}

	// Apply with invalid code
	_, err = ApplyReferral(db, "invalid", "newuser2")
	if err == nil {
		t.Error("expected error for invalid code")
	}
}

func TestApplyReferralMaxUses(t *testing.T) {
	db := newTestDB(t)

	// Create a code with max_uses = 1
	code := "testmax1"
	_, err := db.DB().Exec(
		`INSERT INTO referrals (code, referrer_id, uses, max_uses, created_at) VALUES (?, ?, 0, 1, ?)`,
		code, "referrer2", 1000000,
	)
	if err != nil {
		t.Fatalf("insert referral: %v", err)
	}

	bonus, err := ApplyReferral(db, code, "newuser1")
	if err != nil {
		t.Fatalf("ApplyReferral first: %v", err)
	}
	if bonus != 30 {
		t.Errorf("bonus = %d, want 30", bonus)
	}

	// Second use should fail
	_, err = ApplyReferral(db, code, "newuser2")
	if err == nil {
		t.Error("expected error for max uses exceeded")
	}
}

func TestGetReferralStats(t *testing.T) {
	db := newTestDB(t)

	// Create codes
	code1, _ := CreateReferralCode(db, "referrer1")
	code2, _ := CreateReferralCode(db, "referrer1")

	// Apply referrals
	_, _ = ApplyReferral(db, code1, "newuser1")
	_, _ = ApplyReferral(db, code2, "newuser2")

	stats, err := GetReferralStats(db, "referrer1")
	if err != nil {
		t.Fatalf("GetReferralStats: %v", err)
	}

	totalReferred, _ := stats["totalReferred"].(int)
	if totalReferred != 2 {
		t.Errorf("totalReferred = %d, want 2", totalReferred)
	}

	codes, ok := stats["codes"]
	if !ok {
		t.Error("stats should contain 'codes'")
	}
	_ = codes // just verify it exists
}
