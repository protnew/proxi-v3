package invite

import (
	"testing"
	"time"

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

func TestCreateInvite(t *testing.T) {
	db := newTestDB(t)
	inv, err := CreateInvite(db, "ch1", "", "alice", 10, 3600)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if inv.ID == "" {
		t.Error("invite ID should not be empty")
	}
	if inv.Channel != "ch1" {
		t.Errorf("expected channel ch1, got %s", inv.Channel)
	}
	if inv.MaxUses != 10 {
		t.Errorf("expected max uses 10, got %d", inv.MaxUses)
	}
	t.Logf("Created invite: %s", inv.ID)
}

func TestGetInvite(t *testing.T) {
	db := newTestDB(t)
	created, _ := CreateInvite(db, "ch1", "", "alice", 5, 0)

	got, err := GetInvite(db, created.ID)
	if err != nil {
		t.Fatalf("GetInvite: %v", err)
	}
	if got.Channel != "ch1" {
		t.Errorf("expected ch1, got %s", got.Channel)
	}
	if got.CreatedBy != "alice" {
		t.Errorf("expected alice, got %s", got.CreatedBy)
	}
}

func TestRedeemInvite(t *testing.T) {
	db := newTestDB(t)
	inv, _ := CreateInvite(db, "ch1", "", "alice", 2, 0)

	if err := RedeemInvite(db, inv.ID, "bob"); err != nil {
		t.Fatalf("RedeemInvite: %v", err)
	}

	got, _ := GetInvite(db, inv.ID)
	if got.Uses != 1 {
		t.Errorf("expected 1 use, got %d", got.Uses)
	}

	// Second redeem OK
	RedeemInvite(db, inv.ID, "carol")
	got, _ = GetInvite(db, inv.ID)
	if got.Uses != 2 {
		t.Errorf("expected 2 uses, got %d", got.Uses)
	}
}

func TestRedeemInvite_MaxUses(t *testing.T) {
	db := newTestDB(t)
	inv, _ := CreateInvite(db, "ch1", "", "alice", 1, 0)

	RedeemInvite(db, inv.ID, "bob")
	err := RedeemInvite(db, inv.ID, "carol")
	if err == nil {
		t.Error("should fail: max uses reached")
	}
}

func TestRedeemInvite_Expired(t *testing.T) {
	db := newTestDB(t)
	// Create invite that expired 10 seconds ago
	inv, _ := CreateInvite(db, "ch1", "", "alice", 10, 0)
	// Manually set expires_at in the past
	db.DB().Exec("UPDATE invites SET expires_at = ? WHERE id = ?", time.Now().Unix()-10, inv.ID)

	err := RedeemInvite(db, inv.ID, "bob")
	if err == nil {
		t.Error("should fail: expired")
	}
	t.Logf("Expired invite error (expected): %v", err)
}

func TestRedeemInvite_NotFound(t *testing.T) {
	db := newTestDB(t)
	err := RedeemInvite(db, "nonexistent", "bob")
	if err == nil {
		t.Error("should fail: not found")
	}
}

func TestGenerateInviteURL(t *testing.T) {
	url := GenerateInviteURL("https://proxi.chat", "abc123")
	if url != "https://proxi.chat/invite/abc123" {
		t.Errorf("unexpected URL: %s", url)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()
	if id1 == id2 {
		t.Error("IDs should be unique")
	}
	if len(id1) != 32 {
		t.Errorf("expected 32-char hex ID, got %d chars", len(id1))
	}
}
