package store

import "testing"

func TestRunMigrations(t *testing.T) {
	db, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
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
	v1 := db1.GetSchemaVersion()

	db2, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
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
