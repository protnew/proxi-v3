package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// writeCoreEndpoint atomically publishes the bound loopback endpoint so the
// helper/pipe-client can find the core port without scraping logs (P-C).
func writeCoreEndpoint(dataDir, port string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{
		"endpoint": "127.0.0.1:" + port,
		"pid":      os.Getpid(),
		"ts":       time.Now().Unix(),
	})
	tmp := filepath.Join(dataDir, ".core-endpoint.json.tmp")
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dataDir, "core-endpoint.json"))
}

// removeCoreEndpoint best-effort cleanup on shutdown.
func removeCoreEndpoint(dataDir string) {
	_ = os.Remove(filepath.Join(dataDir, "core-endpoint.json"))
}

// cliBackup creates a VACUUM INTO backup before migrations/mutations (P-C).
func cliBackup() (string, error) {
	dbPath, dataDir := resolveDBPath()
	if dataDir == "" {
		return "", fmt.Errorf("DATA_DIR unset and no project marker found")
	}
	st, err := store.NewStore(dbPath)
	if err != nil {
		return "", err
	}
	defer st.Close()
	dst := filepath.Join(dataDir, "backups",
		fmt.Sprintf("messenger-%d.db", time.Now().Unix()))
	if err := st.Backup(dst); err != nil {
		return "", err
	}
	return dst, nil
}

// cliWipeData removes DB + sidecars + identity keys under DATA_DIR (P-C).
func cliWipeData() error {
	dbPath, dataDir := resolveDBPath()
	if dataDir == "" {
		return fmt.Errorf("DATA_DIR unset and no project marker found")
	}
	victims := []string{
		dbPath, dbPath + "-wal", dbPath + "-shm",
		filepath.Join(dataDir, "private.key"),
		filepath.Join(dataDir, "exitauth.json"),
		filepath.Join(dataDir, "core-endpoint.json"),
	}
	for _, v := range victims {
		if err := os.Remove(v); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("wipe %s: %w", v, err)
		}
	}
	return nil
}
