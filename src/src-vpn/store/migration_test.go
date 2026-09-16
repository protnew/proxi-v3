package store

import (
	"strings"
	"testing"
)

func TestRunMigrations(t *testing.T) {
	db, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	version := db.GetSchemaVersion()
	if version < 1 {
		t.Errorf("expected version >= 1 after init, got %d", version)
	}
	t.Logf("Schema version: %d", version)
}

func TestMigrationsIdempotent(t *testing.T) {
	db1, _ := NewStore(":memory:")
	if db1 == nil {
		t.Fatal("first store failed")
	}
	defer db1.Close()
	v1 := db1.GetSchemaVersion()

	db2, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	v2 := db2.GetSchemaVersion()
	if v1 != v2 {
		t.Errorf("versions should match: %d vs %d", v1, v2)
	}
}

func TestMigrationsCount(t *testing.T) {
	if len(migrations) < 9 {
		t.Errorf("expected at least 9 migrations, got %d", len(migrations))
	}
	for i, m := range migrations {
		if m.Version != i+1 {
			t.Errorf("migration %d has version %d, expected %d", i, m.Version, i+1)
		}
		if m.Name == "" {
			t.Errorf("migration %d has no name", i)
		}
		if m.Up == "" {
			t.Errorf("migration %d has no SQL", i)
		}
	}
}

func TestMigrationUpHasNoDropTable(t *testing.T) {
	for _, m := range migrations {
		if strings.Contains(strings.ToUpper(m.Up), "DROP TABLE") {
			t.Fatalf("migration v%d Up contains destructive DROP TABLE", m.Version)
		}
	}
}

func TestMessagesSoftDeleteAndErasedAtColumns(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, col := range []string{"deleted_at", "erased_at", "is_deleted"} {
		var name string
		err := s.DB().QueryRow(`SELECT name FROM pragma_table_info('messages') WHERE name = ?`, col).Scan(&name)
		if err != nil || name != col {
			t.Fatalf("messages.%s missing: err=%v name=%q", col, err, name)
		}
	}
	if s.GetSchemaVersion() < 15 {
		t.Fatalf("schema version %d want >= 15", s.GetSchemaVersion())
	}
}

func TestFTS5ProbeIsNotADrop(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='messages'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("messages table missing after FTS probe: n=%d err=%v", n, err)
	}
}
