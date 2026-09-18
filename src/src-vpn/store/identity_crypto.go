package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unkillable-messenger/vpn/crypto"
)

// encPrefix marks AES-GCM ciphertext produced by sealField (P6).
// Plaintext legacy rows lack this prefix and are migrated on open.
const encPrefix = "enc1:"

// identityKeyFileSuffix is appended to the SQLite path for the at-rest key
// (lives outside the DB file on purpose).
const identityKeyFileSuffix = ".identity.key"

// loadOrCreateIdentityKey resolves the 32-byte AES key used to encrypt
// identity.nsec / seed_phrase at rest.
//
// Priority:
//  1. UNKILLABLE_IDENTITY_KEY env (64 hex chars)
//  2. UNKILLABLE_IDENTITY_KEY_FILE env (raw 32 bytes or 64 hex)
//  3. sidecar file next to dbPath (created 0600 if missing)
//  4. ephemeral random key for :memory: / empty path
func loadOrCreateIdentityKey(dbPath string) ([32]byte, string, error) {
	var key [32]byte

	if hexKey := strings.TrimSpace(os.Getenv("UNKILLABLE_IDENTITY_KEY")); hexKey != "" {
		raw, err := hex.DecodeString(hexKey)
		if err != nil || len(raw) != 32 {
			return key, "", fmt.Errorf("UNKILLABLE_IDENTITY_KEY must be 64 hex chars")
		}
		copy(key[:], raw)
		return key, "env:UNKILLABLE_IDENTITY_KEY", nil
	}

	if file := strings.TrimSpace(os.Getenv("UNKILLABLE_IDENTITY_KEY_FILE")); file != "" {
		k, err := readIdentityKeyFile(file)
		if err != nil {
			return key, "", err
		}
		return k, file, nil
	}

	if dbPath == "" || dbPath == ":memory:" {
		if _, err := rand.Read(key[:]); err != nil {
			return key, "", fmt.Errorf("generate ephemeral identity key: %w", err)
		}
		return key, ":memory:", nil
	}

	keyPath := dbPath + identityKeyFileSuffix
	if abs, err := filepath.Abs(dbPath); err == nil {
		keyPath = abs + identityKeyFileSuffix
	}

	if _, err := os.Stat(keyPath); err == nil {
		k, err := readIdentityKeyFile(keyPath)
		if err != nil {
			return key, "", err
		}
		return k, keyPath, nil
	}

	if _, err := rand.Read(key[:]); err != nil {
		return key, "", fmt.Errorf("generate identity key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return key, "", fmt.Errorf("mkdir for identity key: %w", err)
	}
	if err := os.WriteFile(keyPath, key[:], 0o600); err != nil {
		return key, "", fmt.Errorf("write identity key file: %w", err)
	}
	return key, keyPath, nil
}

func readIdentityKeyFile(path string) ([32]byte, error) {
	var key [32]byte
	data, err := os.ReadFile(path)
	if err != nil {
		return key, fmt.Errorf("read identity key file %s: %w", path, err)
	}
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 32 {
		copy(key[:], data)
		return key, nil
	}
	if len(data) == 64 {
		raw, err := hex.DecodeString(string(data))
		if err != nil || len(raw) != 32 {
			return key, fmt.Errorf("identity key file %s: expected 32 raw bytes or 64 hex", path)
		}
		copy(key[:], raw)
		return key, nil
	}
	return key, fmt.Errorf("identity key file %s: got %d bytes, want 32 raw or 64 hex", path, len(data))
}

func (s *Store) sealField(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if strings.HasPrefix(plain, encPrefix) {
		return plain, nil
	}
	enc, err := crypto.EncryptMessage(plain, s.identityKey)
	if err != nil {
		return "", fmt.Errorf("seal identity field: %w", err)
	}
	return encPrefix + enc, nil
}

func (s *Store) openField(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, encPrefix) {
		// Legacy plaintext (pre-P6) — returned as-is; migrateIdentityAtRest will seal.
		return stored, nil
	}
	plain, err := crypto.DecryptMessage(strings.TrimPrefix(stored, encPrefix), s.identityKey)
	if err != nil {
		return "", fmt.Errorf("open identity field: %w", err)
	}
	return plain, nil
}

// migrateIdentityAtRest re-encrypts any plaintext nsec/seed_phrase rows in place.
func (s *Store) migrateIdentityAtRest() error {
	var nsec, seed string
	err := s.db.QueryRow(`SELECT nsec, seed_phrase FROM identity WHERE id = 1`).Scan(&nsec, &seed)
	if err != nil {
		// No row yet — nothing to migrate.
		return nil
	}
	needNsec := nsec != "" && !strings.HasPrefix(nsec, encPrefix)
	needSeed := seed != "" && !strings.HasPrefix(seed, encPrefix)
	if !needNsec && !needSeed {
		return nil
	}
	sealedNsec, err := s.sealField(nsec)
	if err != nil {
		return err
	}
	sealedSeed, err := s.sealField(seed)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`UPDATE identity SET nsec = ?, seed_phrase = ? WHERE id = 1`,
		sealedNsec, sealedSeed,
	)
	if err != nil {
		return fmt.Errorf("migrate identity at rest: %w", err)
	}
	return nil
}
