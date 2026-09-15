package store

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// storePath holds the original database path for backup/restore.
// This is set during NewStore and is used by Backup/Restore methods.
var storeMu sync.Mutex

// Backup creates a backup of the SQLite database using VACUUM INTO.
func (s *Store) Backup(dstPath string) error {
	if s.db == nil {
		return fmt.Errorf("store: database not open")
	}

	// Ensure destination directory exists
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("store: create backup dir: %w", err)
	}

	// Use VACUUM INTO (SQLite 3.27+)
	escaped := strings.ReplaceAll(dstPath, "'", "''")
	_, err := s.db.Exec("VACUUM INTO '" + escaped + "'")
	if err != nil {
		return fmt.Errorf("store: VACUUM INTO failed: %w", err)
	}

	return nil
}

// Restore closes the current database, replaces the DB file with the
// backup file at srcPath, and reopens the database connection.
func (s *Store) Restore(srcPath string, dbPath string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	if s.db == nil {
		return fmt.Errorf("store: database not open")
	}

	// Validate source exists
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("store: backup file not found: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("store: backup path is a directory, not a file")
	}

	// Close current DB
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("store: close db: %w", err)
	}
	s.db = nil

	removeSQLiteSidecars(dbPath)

	// Copy backup file to DB path
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("store: open backup: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dbPath)
	if err != nil {
		return fmt.Errorf("store: create db file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("store: copy backup: %w", err)
	}
	dst.Sync()

	// Reopen database
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("store: reopen db: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return fmt.Errorf("store: enable foreign keys: %w", err)
	}

	s.db = db
	return nil
}

func removeSQLiteSidecars(dbPath string) {
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
}

// AutoBackup starts a goroutine that periodically backs up the database
// to the specified directory using VACUUM INTO.
// It uses a context to terminate the loop.
func (s *Store) AutoBackup(ctx context.Context, dir string, interval time.Duration) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		_ = err
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				backupName := fmt.Sprintf("messenger_backup_%s.db", time.Now().Format("20060102_150405"))
				dstPath := filepath.Join(dir, backupName)
				if err := s.Backup(dstPath); err != nil {
					// Silent fail — log if needed
					_ = err
				}
			}
		}
	}()
}
