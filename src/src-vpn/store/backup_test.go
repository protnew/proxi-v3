package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupRestore(t *testing.T) {
	// Create temp directories for DB and backup
	dbDir := t.TempDir()
	backupDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")
	backupPath := filepath.Join(backupDir, "test_backup.db")

	// Create store and save some data
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	msg := Message{
		ID:        "test-backup-1",
		From:      "alice",
		To:        "broadcast",
		Text:      "backup test message",
		Encrypted: false,
		Timestamp: 1700000000,
	}
	if err := s.SaveMessage(msg); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	// Backup
	if err := s.Backup(backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// Verify backup file exists and has content
	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("backup file stat: %v", err)
	}
	if info.Size() == 0 {
		t.Error("backup file is empty")
	}

	// Open backup store and verify data
	s2, err := NewStore(backupPath)
	if err != nil {
		t.Fatalf("NewStore (backup): %v", err)
	}
	defer s2.Close()

	msgs, err := s2.GetMessages(10, 0, "")
	if err != nil {
		t.Fatalf("GetMessages (backup): %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("backup store has no messages")
	}
	found := false
	for _, m := range msgs {
		if m.ID == "test-backup-1" {
			found = true
			if m.Text != "backup test message" {
				t.Errorf("message text mismatch: got %q", m.Text)
			}
			break
		}
	}
	if !found {
		t.Error("saved message not found in backup")
	}
	s.Close()

	// Test Restore
	restorePath := filepath.Join(backupDir, "test_restore.db")
	restoreStore, err := NewStore(restorePath)
	if err != nil {
		t.Fatalf("NewStore (restore target): %v", err)
	}
	// Save different data to the restore target
	restoreMsg := Message{
		ID:        "should-be-overwritten",
		From:      "bob",
		To:        "broadcast",
		Text:      "this should be overwritten",
		Encrypted: false,
		Timestamp: 1700000001,
	}
	if _, err := restoreStore.DB().Exec("PRAGMA journal_mode = WAL"); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}
	if _, err := restoreStore.DB().Exec("PRAGMA wal_autocheckpoint = 0"); err != nil {
		t.Fatalf("disable WAL autocheckpoint: %v", err)
	}
	if err := restoreStore.SaveMessage(restoreMsg); err != nil {
		t.Fatalf("SaveMessage restore target: %v", err)
	}
	walInfo, err := os.Stat(restorePath + "-wal")
	if err != nil {
		t.Fatalf("expected live WAL before restore: %v", err)
	}
	if walInfo.Size() == 0 {
		t.Fatal("expected non-empty WAL before restore")
	}

	// Restore must discard the live DB's WAL/SHM before replacing the main file.
	if err := restoreStore.Restore(backupPath, restorePath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify restored data
	msgs2, err := restoreStore.GetMessages(10, 0, "")
	if err != nil {
		t.Fatalf("GetMessages (restored): %v", err)
	}
	found = false
	for _, m := range msgs2 {
		if m.ID == "test-backup-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("original message not found after restore")
	}
	var integrity string
	if err := restoreStore.DB().QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity_check=%q want ok", integrity)
	}
	// The overwritten message should not be there
	for _, m := range msgs2 {
		if m.ID == "should-be-overwritten" {
			t.Error("overwritten message should not exist after restore")
		}
	}

	restoreStore.Close()
}

func TestRestoreRemovesWalSidecars(t *testing.T) {
	dbDir := t.TempDir()
	backupDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "live.db")
	backupPath := filepath.Join(backupDir, "keep.db")

	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	keep := Message{ID: "keep", From: "alice", To: "broadcast", Text: "from-backup", Timestamp: 1}
	if err := s.SaveMessage(keep); err != nil {
		t.Fatalf("SaveMessage keep: %v", err)
	}
	if err := s.Backup(backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	drop := Message{ID: "drop", From: "alice", To: "broadcast", Text: "after-backup", Timestamp: 2}
	if err := s.SaveMessage(drop); err != nil {
		t.Fatalf("SaveMessage drop: %v", err)
	}

	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"
	if _, err := os.Stat(walPath); err != nil {
		if err := os.WriteFile(walPath, []byte("stale-wal-poison-p1"), 0o600); err != nil {
			t.Fatalf("write wal: %v", err)
		}
	}
	if _, err := os.Stat(shmPath); err != nil {
		if err := os.WriteFile(shmPath, []byte("stale-shm-poison-p1"), 0o600); err != nil {
			t.Fatalf("write shm: %v", err)
		}
	}

	if err := s.Restore(backupPath, dbPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	var integrity string
	if err := s.DB().QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity_check=%q want ok", integrity)
	}

	msgs, err := s.GetMessages(50, 0, "alice")
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	ids := map[string]bool{}
	for _, m := range msgs {
		ids[m.ID] = true
	}
	if !ids["keep"] {
		t.Fatal("backup row missing after restore")
	}
	if ids["drop"] {
		t.Fatal("post-backup row survived restore (stale WAL replay?)")
	}
	s.Close()
}
