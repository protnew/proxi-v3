package store

import (
	"crypto/rand"
	"crypto/sha256"
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

// identityKeyFileSuffix was the pre-P27 sidecar name (dbPath + suffix).
// P27: the key moved OUT of the data dir — see identityKeyPath.
const identityKeyFileSuffix = ".identity.key"

// identityKeyPath returns the P27 key location: per-DB key inside the
// user config dir, keyed by the absolute DB path — outside the data dir
// so a leaked DB file alone does not decrypt.
func identityKeyPath(dbPath string) (string, error) {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		abs = dbPath
	}
	sum := sha256.Sum256([]byte(abs))
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(cfgDir, "proxi", "keys", "identity-"+hex.EncodeToString(sum[:8])+".key"), nil
}

// loadOrCreateIdentityKey resolves the 32-byte AES key used to encrypt
// identity.nsec / seed_phrase at rest.
//
// Priority:
//  1. PROXI_IDENTITY_KEY env (64 hex chars); legacy UNKILLABLE_IDENTITY_KEY
//  2. PROXI_IDENTITY_KEY_FILE env; legacy UNKILLABLE_IDENTITY_KEY_FILE
//  3. per-DB file under user config dir (outside data dir); a legacy
//     sidecar next to the DB is migrated there on first open
//  4. ephemeral random key for :memory: / empty path
func loadOrCreateIdentityKey(dbPath string) ([32]byte, string, error) {
	var key [32]byte

	for _, env := range []string{"PROXI_IDENTITY_KEY", "UNKILLABLE_IDENTITY_KEY"} {
		if hexKey := strings.TrimSpace(os.Getenv(env)); hexKey != "" {
			raw, err := hex.DecodeString(hexKey)
			if err != nil || len(raw) != 32 {
				return key, "", fmt.Errorf("%s must be 64 hex chars", env)
			}
			copy(key[:], raw)
			return key, "env:" + env, nil
		}
	}

	for _, env := range []string{"PROXI_IDENTITY_KEY_FILE", "UNKILLABLE_IDENTITY_KEY_FILE"} {
		if file := strings.TrimSpace(os.Getenv(env)); file != "" {
			k, err := readIdentityKeyFile(file)
			if err != nil {
				return key, "", err
			}
			return k, file, nil
		}
	}

	if dbPath == "" || dbPath == ":memory:" {
		if _, err := rand.Read(key[:]); err != nil {
			return key, "", fmt.Errorf("generate ephemeral identity key: %w", err)
		}
		return key, ":memory:", nil
	}

	keyPath, err := identityKeyPath(dbPath)
	if err != nil {
		return key, "", err
	}

	// Migrate a pre-P27 sidecar key if present.
	legacyPath := dbPath + identityKeyFileSuffix
	if abs, aerr := filepath.Abs(dbPath); aerr == nil {
		legacyPath = abs + identityKeyFileSuffix
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		if k, rerr := readIdentityKeyFile(legacyPath); rerr == nil {
			if werr := writeIdentityKeyFile(keyPath, k); werr == nil {
				_ = os.Remove(legacyPath)
			}
			return k, keyPath, nil
		}
	} else if err == nil {
		k, rerr := readIdentityKeyFile(keyPath)
		if rerr != nil {
			return key, "", rerr
		}
		return k, keyPath, nil
	}

	if _, err := rand.Read(key[:]); err != nil {
		return key, "", fmt.Errorf("generate identity key: %w", err)
	}
	if err := writeIdentityKeyFile(keyPath, key); err != nil {
		return key, "", err
	}
	return key, keyPath, nil
}

func writeIdentityKeyFile(path string, key [32]byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir for identity key: %w", err)
	}
	if err := os.WriteFile(path, key[:], 0o600); err != nil {
		return fmt.Errorf("write identity key file: %w", err)
	}
	return nil
}

func readIdentityKeyFile(path string) ([32]byte, error) {
	var key [32]byte
	data, err := os.ReadFile(path)
	if err != nil {
		return key, fmt.Errorf("read identity key file %s: %w", path, err)
	}
	// Raw 32-byte keys are binary: never TrimSpace — 0x09/0x0a/0x20 etc. are valid key bytes.
	// TrimSpace only for hex text form (64 chars).
	if len(data) == 32 {
		copy(key[:], data)
		return key, nil
	}
	trimmed := []byte(strings.TrimSpace(string(data)))
	if len(trimmed) == 64 {
		raw, err := hex.DecodeString(string(trimmed))
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
