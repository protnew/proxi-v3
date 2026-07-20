package admin

import (
	"context"
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

func newTestPanel(t *testing.T) *Panel {
	t.Helper()
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	return NewPanel(db)
}

func TestGetDashboardStats(t *testing.T) {
	p := newTestPanel(t)
	ctx := context.Background()

	p.db.DB().Exec("INSERT INTO messages (id, sender, recipient, text, timestamp) VALUES (?, ?, ?, ?, ?)", "m1", "alice", "broadcast", "hello", 1000)
	p.db.DB().Exec("INSERT INTO messages (id, sender, recipient, text, timestamp) VALUES (?, ?, ?, ?, ?)", "m2", "bob", "broadcast", "hi", 2000)
	p.db.DB().Exec("INSERT INTO channels (id, name, creator, created_at) VALUES (?, ?, ?, ?)", "ch1", "test", "alice", 1000)
	p.db.DB().Exec("INSERT INTO peers (id, name, public_key, endpoint, allowed_ips) VALUES (?, ?, ?, ?, ?)", "p1", "peer1", "pk1", "1.2.3.4:51820", "10.0.0.0/24")
	p.db.DB().Exec("INSERT INTO peers (id, name) VALUES (?, ?)", "p1", "peer1")

	stats, err := p.GetDashboardStats(ctx)
	if err != nil {
		t.Fatalf("GetDashboardStats: %v", err)
	}
	if stats["messages"].(int) != 2 {
		t.Errorf("expected 2 messages, got %v", stats["messages"])
	}
	if stats["channels"].(int) != 1 {
		t.Errorf("expected 1 channel, got %v", stats["channels"])
	}
	if stats["peers"].(int) != 1 {
		t.Errorf("expected 1 peer, got %v", stats["peers"])
	}
}

func TestListUsers(t *testing.T) {
	p := newTestPanel(t)
	ctx := context.Background()

	p.db.DB().Exec("INSERT INTO messages (id, sender, recipient, text, timestamp) VALUES (?, ?, ?, ?, ?)", "m1", "alice", "broadcast", "hello", 1000)
	p.db.DB().Exec("INSERT INTO messages (id, sender, recipient, text, timestamp) VALUES (?, ?, ?, ?, ?)", "m2", "alice", "broadcast", "world", 2000)
	p.db.DB().Exec("INSERT INTO messages (id, sender, recipient, text, timestamp) VALUES (?, ?, ?, ?, ?)", "m3", "bob", "broadcast", "hi", 3000)

	users, err := p.ListUsers(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	var alice map[string]interface{}
	for _, u := range users {
		if u["user_id"] == "alice" {
			alice = u
			break
		}
	}
	if alice == nil {
		t.Fatal("alice not found")
	}
	if alice["msg_count"].(int) != 2 {
		t.Errorf("alice should have 2 messages, got %v", alice["msg_count"])
	}
	if alice["banned"].(bool) {
		t.Error("alice should not be banned")
	}
}

func TestBanUnban(t *testing.T) {
	p := newTestPanel(t)
	ctx := context.Background()

	expires := time.Now().Add(time.Hour)
	if err := p.BanUser(ctx, "alice", "spam", expires); err != nil {
		t.Fatalf("BanUser: %v", err)
	}

	banned, err := p.IsBanned(ctx, "alice")
	if err != nil {
		t.Fatalf("IsBanned: %v", err)
	}
	if !banned {
		t.Error("alice should be banned")
	}

	if err := p.UnbanUser(ctx, "alice"); err != nil {
		t.Fatalf("UnbanUser: %v", err)
	}

	banned, _ = p.IsBanned(ctx, "alice")
	if banned {
		t.Error("alice should not be banned after unban")
	}
}

func TestIsBanned_Expired(t *testing.T) {
	p := newTestPanel(t)
	ctx := context.Background()

	expires := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC) // clearly in the past
	p.BanUser(ctx, "alice", "old ban", expires)

	banned, _ := p.IsBanned(ctx, "alice")
	if banned {
		t.Error("expired ban should not count")
	}
}

func TestDeleteMessage(t *testing.T) {
	p := newTestPanel(t)
	ctx := context.Background()

	_, err := p.db.DB().Exec("INSERT INTO messages (id, sender, recipient, text, timestamp) VALUES (?, ?, ?, ?, ?)", "msg1", "alice", "broadcast", "bad message", 1000)
	if err != nil {
		t.Fatal(err)
	}

	if err := p.DeleteMessage(ctx, "msg1"); err != nil {
		t.Fatalf("DeleteMessage: %v", err)
	}

	var count int
	p.db.DB().QueryRow("SELECT COUNT(*) FROM messages WHERE id = ?", "msg1").Scan(&count)
	if count != 0 {
		t.Error("message should be deleted")
	}

	err = p.DeleteMessage(ctx, "nonexistent")
	if err == nil {
		t.Error("should error on non-existent message")
	}
}

func TestReports(t *testing.T) {
	p := newTestPanel(t)
	ctx := context.Background()

	if err := p.CreateReport(ctx, 42, "alice", "inappropriate content"); err != nil {
		t.Fatalf("CreateReport: %v", err)
	}

	reports, err := p.ListReports(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0]["status"] != "open" {
		t.Errorf("report should be open, got %v", reports[0]["status"])
	}
	if reports[0]["message_id"].(int64) != 42 {
		t.Errorf("expected message_id 42, got %v", reports[0]["message_id"])
	}

	reportID := reports[0]["id"].(int64)
	if err := p.ResolveReport(ctx, reportID, "deleted", "admin"); err != nil {
		t.Fatalf("ResolveReport: %v", err)
	}

	reports, _ = p.ListReports(ctx, 0, 10)
	if reports[0]["status"] != "deleted" {
		t.Errorf("report should be resolved, got %v", reports[0]["status"])
	}
}

func TestResolveNonExistentReport(t *testing.T) {
	p := newTestPanel(t)
	ctx := context.Background()

	err := p.ResolveReport(ctx, 9999, "dismissed", "admin")
	if err == nil {
		t.Error("should error on non-existent report")
	}
}
