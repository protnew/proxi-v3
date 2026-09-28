//go:build !windows

package main

import "fmt"

func assignTunAddr(name string) error      { return fmt.Errorf("routes unsupported on this OS") }
func setTunDNS(name string) error          { return fmt.Errorf("dns unsupported") }
func clearTunDNS(name string) error        { return nil }
func installSplitRoutes(name string) error { return fmt.Errorf("routes unsupported") }
func removeSplitRoutes(name string) error  { return nil }
