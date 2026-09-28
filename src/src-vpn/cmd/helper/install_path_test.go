package main

import (
	"runtime"
	"testing"
)

func TestInstallPath_RefusesDownloads(t *testing.T) {
	// pathUnderAdminDir matches Windows path semantics (filepath.Separator);
	// on linux CI the C:\ prefixes never match — nothing to assert there.
	if runtime.GOOS != "windows" {
		t.Skip("windows path semantics only")
	}
	roots := []string{`C:\Program Files`, `C:\Program Files (x86)`}
	if pathUnderAdminDir(`C:\Users\x\Downloads\proxi04-vpn-helper.exe`, roots) {
		t.Fatal("downloads allowed")
	}
	if !pathUnderAdminDir(`C:\Program Files\Proxi\proxi04-vpn-helper.exe`, roots) {
		t.Fatal("program files denied")
	}
}
