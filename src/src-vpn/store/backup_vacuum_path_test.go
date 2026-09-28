package store

import (
	"path/filepath"
	"testing"
)

func TestValidateVacuumPathAcceptsUnderBase(t *testing.T) {
	base := t.TempDir()
	dst := filepath.Join(base, "backup.db")
	got, err := validateVacuumPath(dst, base)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == "" {
		t.Fatal("empty abs")
	}
}

func TestValidateVacuumPathRejectsEscape(t *testing.T) {
	base := t.TempDir()
	dst := filepath.Join(base, "..", "outside.db")
	if _, err := validateVacuumPath(dst, base); err == nil {
		t.Fatal("expected escape to fail")
	}
}

func TestValidateVacuumPathRejectsEmpty(t *testing.T) {
	if _, err := validateVacuumPath("  ", t.TempDir()); err == nil {
		t.Fatal("expected empty to fail")
	}
}