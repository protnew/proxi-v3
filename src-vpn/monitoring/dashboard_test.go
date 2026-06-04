package monitoring

import (
	"os"
	"testing"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

func TestCollectDashboardData(t *testing.T) {
	m := NewMetrics()
	m.SetGauge("messages_per_sec", 42.5)
	m.SetGauge("active_users", 10)
	m.SetGauge("active_streams", 3)
	m.SetGauge("storage_used_bytes", 1_000_000)
	m.SetGauge("vpn_sessions", 5)
	m.IncrementCounter("messages_sent")
	m.IncrementCounter("messages_sent")
	m.IncrementCounter("messages_sent")
	m.IncrementCounter("errors")
	m.ObserveLatency("api_request", 100*time.Millisecond)
	m.ObserveLatency("api_request", 200*time.Millisecond)

	// Open a temporary DB
	dbPath := t.TempDir() + "/test.db"
	db, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer db.Close()

	data := CollectDashboardData(m, db, nil)

	if data.MessagesPerSec != 42.5 {
		t.Errorf("MessagesPerSec = %v, want 42.5", data.MessagesPerSec)
	}
	if data.ActiveUsers != 10 {
		t.Errorf("ActiveUsers = %d, want 10", data.ActiveUsers)
	}
	if data.ActiveStreams != 3 {
		t.Errorf("ActiveStreams = %d, want 3", data.ActiveStreams)
	}
	if data.StorageUsed != 1_000_000 {
		t.Errorf("StorageUsed = %d, want 1000000", data.StorageUsed)
	}
	if data.VPNSessions != 5 {
		t.Errorf("VPNSessions = %d, want 5", data.VPNSessions)
	}
	if data.AvgLatency != 0.15 { // (100ms + 200ms) / 2 = 150ms = 0.15s
		t.Errorf("AvgLatency = %v, want 0.15", data.AvgLatency)
	}
	// ErrorRate = 1 error / 3 messages
	if data.ErrorRate < 0.33 || data.ErrorRate > 0.34 {
		t.Errorf("ErrorRate = %v, want ~0.333", data.ErrorRate)
	}
	if data.Uptime < 0 {
		t.Errorf("Uptime = %d, want >= 0", data.Uptime)
	}
	if data.TopChannels == nil {
		t.Error("TopChannels should be initialized (empty slice), not nil")
	}
	if data.SystemHealth == nil {
		t.Error("SystemHealth should be populated")
	}
}

func TestCollectDashboardData_NilMetrics(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	db, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	data := CollectDashboardData(nil, db, nil)
	if data.MessagesPerSec != 0 {
		t.Errorf("MessagesPerSec = %v, want 0", data.MessagesPerSec)
	}
}

func TestCollectDashboardData_NilDB(t *testing.T) {
	m := NewMetrics()
	m.SetGauge("active_users", 5)

	data := CollectDashboardData(m, nil, nil)
	if data.ActiveUsers != 5 {
		t.Errorf("ActiveUsers = %d, want 5", data.ActiveUsers)
	}
}

func TestHealthCheckWithDeps_AllOK(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	db, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	health := HealthCheckWithDeps(db)

	if health["database"] != "ok" {
		t.Errorf("database = %q, want %q", health["database"], "ok")
	}
	if health["storage"] != "ok" {
		t.Errorf("storage = %q, want %q", health["storage"], "ok")
	}
	if health["memory"] != "ok" {
		t.Errorf("memory = %q, want %q", health["memory"], "ok")
	}
}

func TestHealthCheckWithDeps_DBDown(t *testing.T) {
	// Create a DB, then close it to simulate "down"
	dbPath := t.TempDir() + "/test.db"
	db, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	health := HealthCheckWithDeps(db)
	if health["database"] == "ok" {
		t.Errorf("database should not be ok when DB is closed, got %q", health["database"])
	}

	// Also test with nil store
	health2 := HealthCheckWithDeps(nil)
	if health2["database"] == "ok" {
		t.Error("database should not be ok when store is nil")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
