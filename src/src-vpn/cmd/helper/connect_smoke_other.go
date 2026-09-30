//go:build !windows

package main

import "fmt"

func runConnectSmoke(hold, noRoute, adapterOnly bool) error {
	return fmt.Errorf("connect-smoke is windows-only")
}
