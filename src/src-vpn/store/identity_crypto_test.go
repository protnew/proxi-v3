package store

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setTempConfigDir points os.UserConfigDir (AppData / XDG_CONFIG_HOME)
// at a temp dir so key files never touch the real user profile in tests.
func setTempConfigDir(t *testing.T) string {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("AppData", cfg)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	return cfg
}

func TestIdentityEncryptedAtRest(t *testing.T) {
	dir := t.TempDir()
	cfgDir := setTempConfigDir(t)
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

	// P27: key file exists OUTSIDE the data dir (user config dir).
	keyPath := s.keyPath
	if keyPath == "" || strings.HasPrefix(keyPath, dir) {
		t.Fatalf("key must live outside data dir, got %q", keyPath)
	}
	if !strings.HasPrefix(keyPath, cfgDir) {
		t.Fatalf("expected key under config dir %s, got %q", cfgDir, keyPath)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("expected identity key file at %s: %v", keyPath, err)
	}
	if _, err := os.Stat(dbPath + identityKeyFileSuffix); err == nil {
		t.Fatal("legacy sidecar key still present next to DB")
	}
}

func TestIdentityKeySidecarMigration(t *testing.T) {
	dir := t.TempDir()
	cfgDir := setTempConfigDir(t)
	dbPath := filepath.Join(dir, "migrate.db")

	// Simulate a pre-P27 deployment: sidecar key next to the DB.
	legacyKey := bytes.Repeat([]byte{0xAB}, 32)
	legacyPath := dbPath + identityKeyFileSuffix
	if err := os.WriteFile(legacyPath, legacyKey, 0o600); err != nil {
		t.Fatalf("write legacy sidecar: %v", err)
	}

	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	if !strings.HasPrefix(s.keyPath, cfgDir) {
		t.Fatalf("key not migrated to config dir: %q", s.keyPath)
	}
	if s.identityKey != [32]byte(legacyKey) {
		t.Fatal("migrated key bytes differ from legacy sidecar")
	}
	if _, err := os.Stat(legacyPath); err == nil {
		t.Fatal("legacy sidecar not removed after migration")
	}
}

func TestIdentityPlaintextMigration(t *testing.T) {
	dir := t.TempDir()
	setTempConfigDir(t)
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
	t.Setenv("PROXI_IDENTITY_KEY", hexKey)
	defer t.Setenv("PROXI_IDENTITY_KEY", "")

	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()
	if s.keyPath != "env:PROXI_IDENTITY_KEY" {
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

func TestIdentityKeyFileBinaryWhitespace(t *testing.T) {
	// Regression: raw key files must accept edge bytes that TrimSpace would strip.
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "k.identity.key")
	var raw [32]byte
	raw[0] = 0x20  // space
	raw[31] = 0x0a // newline
	for i := 1; i < 31; i++ {
		raw[i] = byte(i)
	}
	if err := os.WriteFile(keyPath, raw[:], 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readIdentityKeyFile(keyPath)
	if err != nil {
		t.Fatalf("readIdentityKeyFile: %v", err)
	}
	if got != raw {
		t.Fatalf("key mismatch after read")
	}
}
