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
	restoreStore.SaveMessage(restoreMsg)

	// Restore from backup
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
	// The overwritten message should not be there
	for _, m := range msgs2 {
		if m.ID == "should-be-overwritten" {
			t.Error("overwritten message should not exist after restore")
		}
	}

	restoreStore.Close()
}
