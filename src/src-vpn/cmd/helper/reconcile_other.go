//go:build !windows

package main

// Non-windows stubs: no WFP/Wintun on other platforms.
func tunnelEngaged() bool                         { return false }
func reassertSplitRoutes()                        {}
func assertNoSplitRoutes(name string) error       { return nil }
