//go:build !windows

package main

import "fmt"

func runDiag() (string, error) {
	return "", fmt.Errorf("--diag is windows-only")
}
