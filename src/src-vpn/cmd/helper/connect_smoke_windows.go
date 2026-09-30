//go:build windows

package main

func runConnectSmoke(hold, noRoute, adapterOnly bool) error {
	return runTunnelPipeline(tunnelOpt{Exit: smokeExit, Hold: hold, NoRoute: noRoute, AdapterOnly: adapterOnly})
}
