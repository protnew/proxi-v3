//go:build !windows

package main

import "fmt"

func installHelperService(exe string) error {
	return fmt.Errorf("service install is windows-only")
}

func probeSCM() error {
	return fmt.Errorf("scm is windows-only")
}

func uninstallHelperService() error {
	return fmt.Errorf("service install is windows-only")
}
