//go:build !windows

package main

import "fmt"

// selftestACL is a windows-runner live check (TZ §2.2) — nothing to do here.
func selftestACL() error {
	return fmt.Errorf("selftest-acl is windows-only")
}
