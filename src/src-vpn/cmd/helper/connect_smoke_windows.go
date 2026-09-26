//go:build windows

package main

func runConnectSmoke(hold bool) error {
	return runTunnelPipeline(tunnelOpt{Exit: smokeExit, Hold: hold})
}
