//go:build !windows

package main

import "fmt"

func runHelperService() error {
	emit("helper-service")
	return fmt.Errorf("service run is windows-only")
}
