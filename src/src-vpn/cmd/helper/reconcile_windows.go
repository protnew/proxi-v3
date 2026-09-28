//go:build windows

package main

import (
	"os/exec"
	"strings"

	"github.com/tailscale/wf"
)

func dhcpMatches(layerIndex int) []*wf.Match {
	port := uint16(67)
	if layerIndex == 1 || layerIndex == 3 {
		port = 547
	}
	if layerIndex >= 2 {
		return []*wf.Match{{
			Field: wf.FieldIPRemotePort,
			Op:    wf.MatchTypeEqual,
			Value: uint16(1),
		}}
	}
	return []*wf.Match{
		{Field: wf.FieldIPProtocol, Op: wf.MatchTypeEqual, Value: wf.IPProtoUDP},
		{Field: wf.FieldIPRemotePort, Op: wf.MatchTypeEqual, Value: port},
	}
}

func tunnelEngaged() bool {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	return liveSess != nil && liveSess.routesOn && !liveSess.closed
}

func reassertSplitRoutes() {
	tunnelMu.Lock()
	name := ""
	if liveSess != nil {
		name = liveSess.name
	}
	tunnelMu.Unlock()
	if name == "" {
		emit("power-resume skip no adapter")
		return
	}
	if err := installSplitRoutes(name); err != nil {
		emit("power-resume reassert-fail " + err.Error())
		return
	}
	emit("power-resume routes-reasserted " + name)
}

func reconcileOnStart() {
	_, n, err := enumKillSwitch()
	if err != nil {
		emit("reconcile enum-fail " + err.Error())
		n = 0
	}
	routeOut, _ := exec.Command("route", "print").CombinedOutput()
	var routes []string
	for _, line := range strings.Split(string(routeOut), "\n") {
		line = strings.TrimSpace(line)
		if routeIsSplit(line) {
			routes = append(routes, line)
		}
	}
	rep := reconcileOrphans(n, routes, nil)
	for _, action := range rep.Actions {
		emit("reconcile " + action)
	}
	if n > 0 && !tunnelEngaged() {
		if err := removeKillSwitch(); err != nil {
			emit("reconcile filter-remove-fail " + err.Error())
		}
	}
	if len(routes) > 0 {
		_ = removeSplitRoutes(tunAdapter)
	}
	if len(rep.Actions) == 0 {
		emit("reconcile clean")
	}
}
