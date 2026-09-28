package main

import (
	"os"
	"path/filepath"
)

// resolveDBPath returns the SQLite file and data directory.
// Empty DB_PATH no longer follows process cwd (that created two messenger.db).
// Default: <directory-with-go.mod>/data/messenger.db.
func resolveDBPath() (dbPath, dataDir string) {
	dataDir = os.Getenv("DATA_DIR")
	dbPath = os.Getenv("DB_PATH")
	if dataDir == "" {
		dataDir = findModuleDataDir()
	}
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "messenger.db")
	}
	return dbPath, dataDir
}

func findModuleDataDir() string {
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		dir := start
		for i := 0; i < 12; i++ {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return filepath.Join(dir, "data")
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	// P-C: no project marker -> fail closed. A silent cwd-relative data dir
	// made two messenger.db files once; an installed exe must get DATA_DIR
	// (ProgramData) explicitly.
	return ""
}
