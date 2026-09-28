//go:build !windows

// Package winac — no-op stubs on non-Windows (TZ-FINAL §2.1–2.3).
package winac

func ApplyProgramDataDACL(path string) error { return nil }
func ApplyAdminOnlyACL(path string) error    { return nil }
func ApplyUserKeyACL(path string) error      { return nil }
