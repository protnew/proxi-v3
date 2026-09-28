package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/unkillable-messenger/vpn/winac"
)

var helperLogPath string

// TZ-FINAL 2.2: size-based rotation, keep logMaxFiles-1 backups.
const (
	logMaxBytes = 1 << 20 // 1 MB
	logMaxFiles = 4       // live + .1 .. .3
)

// rotateLog shifts name → name.1 → … → name.(logMaxFiles-1), drops the oldest,
// keeping logMaxFiles files total (live + .1 … .3).
func rotateLog(path string) {
	for i := logMaxFiles - 1; i >= 1; i-- {
		older := fmt.Sprintf("%s.%d", path, i)
		newer := path
		if i > 1 {
			newer = fmt.Sprintf("%s.%d", path, i-1)
		}
		_ = os.Rename(newer, older)
	}
}

func emit(line string) {
	fmt.Println(line)
	seen := map[string]bool{}
	var paths []string
	if helperLogPath != "" {
		paths = append(paths, helperLogPath)
	}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "proxi04-helper-run.log"))
	}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		// TZ-FINAL 2.2: rotate oversized logs before appending.
		if st, err := os.Stat(path); err == nil && st.Size() >= logMaxBytes {
			rotateLog(path)
		}
		created := false
		if _, err := os.Stat(path); os.IsNotExist(err) {
			created = true
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			continue
		}
		fmt.Fprintln(f, line)
		f.Close()
		if created {
			// TZ-FINAL 2.2: machine logs are for SYSTEM/Admins only.
			_ = winac.ApplyAdminOnlyACL(path)
		}
	}
}

func fail(err error) {
	if err == nil {
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, err.Error())
	emit("ERROR " + err.Error())
	os.Exit(1)
}
