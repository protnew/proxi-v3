//go:build !windows

package main

import "fmt"

func runConnectSmoke(hold bool) error {
	return fmt.Errorf("connect-smoke is windows-only")
}
