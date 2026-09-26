package main

import (
	"os"
	"path/filepath"
)

func installDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

func removeInstallArtifacts(dir string) {
	var stuck []string
	for _, name := range []string{"helper.exe", "pipeclient.exe", "wintun.dll", "proxi-core.exe"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := os.Remove(p); err != nil {
			alt := p + ".deleting"
			_ = os.Remove(alt)
			if err2 := os.Rename(p, alt); err2 == nil {
				stuck = append(stuck, alt)
				continue
			}
			stuck = append(stuck, p)
		}
	}
	if len(stuck) > 0 {
		scheduleInstallDelete(stuck)
	}
}
