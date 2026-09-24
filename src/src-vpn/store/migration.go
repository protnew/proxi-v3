package store

import (
	"strings"
	"database/sql"
	"fmt"
	"log"
)

// migration defines a single versioned database migration.
type migration struct {
	Version int
	Name    string
	Up      string // SQL to apply
}

// migrations is the ordered list of all migrations.
// Each migration runs exactly once. Version is tracked in schema_version table.
var migrations = append(append([]migration{}, migrationsCore...), migrationsFeatures...)

// isBenignSchemaErr: column/table already present (partial legacy DBs).
func isBenignSchemaErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column name") ||
		strings.Contains(msg, "already exists")
}
// runMigrations applies all pending migrations in order.
func runMigrations(db *sql.DB) error {
	// Create schema_version tracking table
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	// Get current version
	var currentVersion int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}
		log.Printf("📦 Migration v%d: %s", m.Version, m.Name)

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration v%d: %w", m.Version, err)
		}

		if _, err := tx.Exec(m.Up); err != nil {
			if !isBenignSchemaErr(err) {
				tx.Rollback()
				return fmt.Errorf("migration v%d (%s): %w", m.Version, m.Name, err)
			}
			log.Printf("⚠️  Migration v%d (%s): benign schema skip: %v", m.Version, m.Name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_version (version, name) VALUES (?, ?)", m.Version, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration v%d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration v%d: %w", m.Version, err)
		}
		log.Printf("✅ Migration v%d applied", m.Version)
	}

	// FTS5 probe lives outside migration Up: CREATE IF NOT EXISTS, ignore errors, never DROP.
	tryEnsureFTS5(db)
	// Post-migration column repair: DBs that recorded a differently-named
	// v15 (see live messenger.db: "push_subscriptions_and_identity_owner")
	// never got the soft-delete columns. Ensure them idempotently.
	ensureColumn(db, "messages", "is_deleted", "INTEGER NOT NULL DEFAULT 0")
	ensureColumn(db, "messages", "deleted_at", "INTEGER NOT NULL DEFAULT 0")
	ensureColumn(db, "messages", "erased_at", "INTEGER NOT NULL DEFAULT 0")
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_is_deleted ON messages(is_deleted)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_deleted_at ON messages(deleted_at)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_erased_at ON messages(erased_at)`)
	return nil
}

// ensureColumn adds a column only when PRAGMA table_info shows it missing —
// safe on DBs where the same-named migration was recorded but partially applied.
func ensureColumn(db *sql.DB, table, column, decl string) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			if name == column {
				return // already present
			}
		}
	}
	if _, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + decl); err != nil {
		log.Printf("⚠️  ensureColumn %s.%s: %v", table, column, err)
	}
}

func tryEnsureFTS5(db *sql.DB) {
	// Capability probe only (separate Exec, errors ignored). Never DROP.
	// Do not create product messages_fts here: it has no content sync and SearchMessagesFTS
	// would stop falling back to LIKE.
	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS fts5_capability_probe USING fts5(x)`)
}

// GetSchemaVersion returns the current schema version.
func (s *Store) GetSchemaVersion() int {
	var v int
	s.DB().QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&v)
	return v
}
