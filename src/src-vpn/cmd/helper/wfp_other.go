//go:build !windows

package main

import "fmt"

func installKillSwitch(tunLUID uint64) error {
	return fmt.Errorf("wfp %s is windows-only", WFPSublayerName)
}

func removeKillSwitch() error {
	return fmt.Errorf("wfp %s is windows-only", WFPSublayerName)
}

func runKillSwitchTest() error {
	return fmt.Errorf("wfp %s is windows-only", WFPSublayerName)
}
