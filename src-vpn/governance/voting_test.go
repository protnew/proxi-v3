package governance

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

func TestCreateVote(t *testing.T) {
	db := newTestDB(t)
	vote, err := CreateVote(db, "ch1", "alice", "Delete channel?", []string{"yes", "no"}, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if vote.ID == "" {
		t.Error("vote ID should be set")
	}
	if len(vote.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(vote.Options))
	}
}

func TestCastAndGetResults(t *testing.T) {
	db := newTestDB(t)
	vote, _ := CreateVote(db, "ch1", "alice", "Question?", []string{"a", "b"}, 24*time.Hour)
	CastVote(db, vote.ID, "alice", "a")
	CastVote(db, vote.ID, "bob", "a")
	CastVote(db, vote.ID, "charlie", "b")

	results, err := GetVoteResults(db, vote.ID)
	if err != nil {
		t.Fatal(err)
	}
	if results["a"] != 2 {
		t.Errorf("expected a=2, got %d", results["a"])
	}
	if results["b"] != 1 {
		t.Errorf("expected b=1, got %d", results["b"])
	}
}

func TestCloseExpiredVotes(t *testing.T) {
	db := newTestDB(t)
	CreateVote(db, "ch1", "alice", "Q?", []string{"y", "n"}, 24*time.Hour)
	closed, _ := CloseExpiredVotes(db)
	if closed != 0 {
		t.Error("no votes should be closed yet")
	}
}
