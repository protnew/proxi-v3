package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityEncryptedAtRest(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	npub := "npub1testpubkeyxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	nsec := "nsec1testsecretkeymustnotappearplaintextindumpx"
	seed := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	if err := s.SaveIdentity(npub, nsec, seed); err != nil {
		t.Fatalf("SaveIdentity: %v", err)
	}

	// Round-trip API still returns plaintext to callers.
	gotNpub, gotNsec, gotSeed, err := s.LoadIdentity()
	if err != nil {
		t.Fatalf("LoadIdentity: %v", err)
	}
	if gotNpub != npub || gotNsec != nsec || gotSeed != seed {
		t.Fatalf("round-trip mismatch: npub=%q nsec=%q seed=%q", gotNpub, gotNsec, gotSeed)
	}

	// Raw DB dump must not contain plaintext secrets.
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read db: %v", err)
	}
	dump := string(raw)
	if strings.Contains(dump, nsec) {
		t.Fatal("plaintext nsec found in SQLite file")
	}
	if strings.Contains(dump, seed) {
		t.Fatal("plaintext seed_phrase found in SQLite file")
	}
	if strings.Contains(dump, "abandon abandon") {
		t.Fatal("plaintext seed words found in SQLite file")
	}

	var storedNsec, storedSeed string
	if err := s.db.QueryRow(`SELECT nsec, seed_phrase FROM identity WHERE id = 1`).Scan(&storedNsec, &storedSeed); err != nil {
		t.Fatalf("raw select: %v", err)
	}
	if !strings.HasPrefix(storedNsec, encPrefix) {
		t.Fatalf("stored nsec missing enc prefix: %q", truncate(storedNsec, 20))
	}
	if !strings.HasPrefix(storedSeed, encPrefix) {
		t.Fatalf("stored seed missing enc prefix: %q", truncate(storedSeed, 20))
	}

	// Key file exists outside DB.
	keyPath := dbPath + identityKeyFileSuffix
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("expected identity key file at %s: %v", keyPath, err)
	}
}

func TestIdentityPlaintextMigration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")

	// Create store, then inject plaintext directly to simulate pre-P6 row.
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	nsecPlain := "nsec1legacyplaintextsecretxxxxxxxxxxxxxxxxxxxx"
	seedPlain := "legacy seed phrase words must migrate"
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO identity (id, npub, nsec, seed_phrase) VALUES (1, ?, ?, ?)`,
		"npub1legacy", nsecPlain, seedPlain,
	)
	if err != nil {
		t.Fatalf("inject plaintext: %v", err)
	}
	s.Close()

	// Re-open triggers migrateIdentityAtRest.
	s2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	var storedNsec, storedSeed string
	if err := s2.db.QueryRow(`SELECT nsec, seed_phrase FROM identity WHERE id = 1`).Scan(&storedNsec, &storedSeed); err != nil {
		t.Fatalf("raw select: %v", err)
	}
	if !strings.HasPrefix(storedNsec, encPrefix) || !strings.HasPrefix(storedSeed, encPrefix) {
		t.Fatalf("migration did not seal: nsec=%q seed=%q", storedNsec, storedSeed)
	}

	_, gotNsec, gotSeed, err := s2.LoadIdentity()
	if err != nil {
		t.Fatalf("LoadIdentity: %v", err)
	}
	if gotNsec != nsecPlain || gotSeed != seedPlain {
		t.Fatalf("decrypted mismatch: %q / %q", gotNsec, gotSeed)
	}

	raw, _ := os.ReadFile(dbPath)
	if strings.Contains(string(raw), nsecPlain) {
		t.Fatal("plaintext nsec still in DB after migration")
	}
}

func TestIdentityKeyFromEnv(t *testing.T) {
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	t.Setenv("UNKILLABLE_IDENTITY_KEY", hexKey)
	defer t.Setenv("UNKILLABLE_IDENTITY_KEY", "")

	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()
	if s.keyPath != "env:UNKILLABLE_IDENTITY_KEY" {
		t.Fatalf("expected env key path, got %q", s.keyPath)
	}
	if err := s.SaveIdentity("npub1", "nsec1abc", "seed"); err != nil {
		t.Fatal(err)
	}
	_, nsec, seed, err := s.LoadIdentity()
	if err != nil || nsec != "nsec1abc" || seed != "seed" {
		t.Fatalf("round-trip failed: %v %q %q", err, nsec, seed)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
