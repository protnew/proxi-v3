package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveInstallArtifacts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"proxi04-vpn-helper.exe", "proxi04-pipe-client.exe", "wintun.dll", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	removeInstallArtifacts(dir)
	for _, name := range []string{"proxi04-vpn-helper.exe", "proxi04-pipe-client.exe", "wintun.dll"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatal(name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatal(err)
	}
}
