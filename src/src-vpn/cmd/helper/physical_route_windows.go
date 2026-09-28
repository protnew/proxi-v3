//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func physicalDefault() (alias, hop string, index uint32, err error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Where-Object { $_.NextHop -ne '0.0.0.0' } | Sort-Object RouteMetric | Select-Object -First 1 | ForEach-Object { $_.InterfaceAlias + '|' + $_.NextHop + '|' + $_.InterfaceIndex }")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", 0, fmt.Errorf("route_add_failed: %w %s", err, strings.TrimSpace(string(out)))
	}
	alias, hop, index, ok := parsePhysLine(strings.TrimSpace(string(out)))
	if !ok {
		return "", "", 0, fmt.Errorf("route_add_failed: no physical default")
	}
	return alias, hop, index, nil
}
