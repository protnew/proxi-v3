package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStageInstall_WritesManifestAndRefusesBadDest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "helper-src.exe")
	if err := os.WriteFile(src, []byte("helper-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "Program Files", "Proxi")
	man, err := stageInstall(dest, src, "")
	if err != nil {
		t.Fatal(err)
	}
	if man.HelperSHA256 == "" {
		t.Fatal("empty sha")
	}
	if _, err := os.Stat(filepath.Join(dest, "install-manifest.json")); err != nil {
		t.Fatal(err)
	}
	if pathUnderAdminDir(filepath.Join(dir, "Downloads", "helper.exe"), []string{filepath.Join(dir, "Program Files")}) {
		t.Fatal("downloads accepted")
	}
}
