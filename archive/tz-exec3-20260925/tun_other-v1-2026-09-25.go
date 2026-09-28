//go:build !windows

package main

import "fmt"

func startWintun(name string, mtu int) error {
	return fmt.Errorf("wintun is windows-only")
}
