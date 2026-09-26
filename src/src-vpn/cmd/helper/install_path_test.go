package main

import "testing"

func TestInstallPath_RefusesDownloads(t *testing.T) {
	roots := []string{`C:\Program Files`, `C:\Program Files (x86)`}
	if pathUnderAdminDir(`C:\Users\x\Downloads\proxi04-vpn-helper.exe`, roots) {
		t.Fatal("downloads allowed")
	}
	if !pathUnderAdminDir(`C:\Program Files\Proxi\proxi04-vpn-helper.exe`, roots) {
		t.Fatal("program files denied")
	}
}
