// P30/VACUUM: path must be under DATA_DIR; reject .. ; retain last N backups.
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

// validateVacuumPath (P30) rejects path traversal and paths outside allowedDir.
func validateVacuumPath(dstPath string, allowedDir string) (string, error) {
	if strings.TrimSpace(dstPath) == "" {
		return "", fmt.Errorf("store: empty VACUUM destination")
	}
	if strings.Contains(dstPath, "\x00") {
		return "", fmt.Errorf("store: NUL in VACUUM path")
	}
	if strings.Contains(dstPath, "..") {
		return "", fmt.Errorf("store: VACUUM path contains ..")
	}
	clean := filepath.Clean(dstPath)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("store: resolve VACUUM path: %w", err)
	}
	base := allowedDir
	if base == "" {
		base = filepath.Dir(abs)
	}
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("store: resolve VACUUM base: %w", err)
	}
	sep := string(os.PathSeparator)
	if abs != baseAbs && !strings.HasPrefix(abs, baseAbs+sep) {
		return "", fmt.Errorf("store: VACUUM path escapes allowed dir")
	}
	if strings.Contains(abs, ".."+sep) {
		return "", fmt.Errorf("store: VACUUM path contains ..")
	}
	return abs, nil
}


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
	_, err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", strings.ReplaceAll(dstPath, "'", "''")))
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

	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("store: backup file not found: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("store: backup path is a directory, not a file")
	}

	srcAbs, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("store: resolve backup path: %w", err)
	}
	dbAbs, err := filepath.Abs(dbPath)
	if err != nil {
		return fmt.Errorf("store: resolve db path: %w", err)
	}
	if srcAbs == dbAbs {
		return fmt.Errorf("store: backup and database paths must differ")
	}

	// Stage and fsync the complete backup before touching the live database.
	tmpPath := dbPath + ".restore-tmp"
	_ = os.Remove(tmpPath)
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("store: open backup: %w", err)
	}
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		src.Close()
		return fmt.Errorf("store: create staged restore: %w", err)
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, src); err != nil {
		src.Close()
		tmp.Close()
		return fmt.Errorf("store: stage backup: %w", err)
	}
	if err := src.Close(); err != nil {
		tmp.Close()
		return fmt.Errorf("store: close backup: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("store: sync staged restore: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store: close staged restore: %w", err)
	}

	// P14 (2026-09-20): integrity_check на СТЕЙДЖ-файле до подмены живой БД —
	// битый бэкап не должен убивать рабочую базу.
	{
		stage, err := sql.Open("sqlite", tmpPath+"?mode=ro&_journal_mode=DELETE")
		if err != nil {
			return fmt.Errorf("store: open staged restore: %w", err)
		}
		var stageIntegrity string
		scanErr := stage.QueryRow("PRAGMA integrity_check").Scan(&stageIntegrity)
		stage.Close()
		if scanErr != nil {
			return fmt.Errorf("store: staged integrity check: %w", scanErr)
		}
		if stageIntegrity != "ok" {
			return fmt.Errorf("store: staged integrity check failed: %s", stageIntegrity)
		}
	}

	if err := s.db.Close(); err != nil {
		return fmt.Errorf("store: close db: %w", err)
	}
	s.db = nil

	// SQLite WAL sidecars belong to the old main DB. They must be removed before
	// replacing it or stale frames can be replayed into the restored database.
	for _, sidecar := range []string{dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("store: remove stale sidecar %s: %w", filepath.Base(sidecar), err)
		}
	}
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("store: remove old db: %w", err)
	}
	if err := os.Rename(tmpPath, dbPath); err != nil {
		return fmt.Errorf("store: install restored db: %w", err)
	}
	cleanupTmp = false

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("store: reopen db: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return fmt.Errorf("store: enable foreign keys: %w", err)
	}
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		db.Close()
		return fmt.Errorf("store: restored integrity check: %w", err)
	}
	if integrity != "ok" {
		db.Close()
		return fmt.Errorf("store: restored integrity check failed: %s", integrity)
	}

	s.db = db
	return nil
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
