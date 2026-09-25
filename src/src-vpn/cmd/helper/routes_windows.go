//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func runNetsh(args ...string) error {
	cmd := exec.Command("netsh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func assignTunAddr(name string) error {
	if err := runNetsh("interface", "ip", "set", "address", "name="+name, "static", "10.7.0.2", "255.255.255.255"); err != nil {
		return err
	}
	_ = runNetsh("interface", "ipv4", "set", "subinterface", name, "mtu=1280", "store=active")
	_ = runNetsh("interface", "ipv6", "add", "address", name, "fd00:7::2/128")
	return nil
}

func setTunDNS(name string) error {
	return runNetsh("interface", "ip", "set", "dns", "name="+name, "static", "10.7.0.1")
}

func clearTunDNS(name string) error {
	_ = runNetsh("interface", "ip", "set", "dns", "name="+name, "dhcp")
	_ = runNetsh("interface", "ip", "delete", "address", "name="+name, "addr=10.7.0.2")
	_ = runNetsh("interface", "ipv6", "delete", "address", name, "fd00:7::2")
	return nil
}

func installSplitRoutes(name string) error {
	v4 := [][]string{
		{"interface", "ipv4", "add", "route", "0.0.0.0/1", name, "10.7.0.1", "metric=1", "store=active"},
		{"interface", "ipv4", "add", "route", "128.0.0.0/1", name, "10.7.0.1", "metric=1", "store=active"},
		{"interface", "ipv4", "add", "route", "1.1.1.1/32", name, "10.7.0.1", "metric=1", "store=active"},
	}
	for _, a := range v4 {
		if err := runNetsh(a...); err != nil {
			return err
		}
	}
	v6 := [][]string{
		{"interface", "ipv6", "add", "route", "::/1", name, "metric=1", "store=active"},
		{"interface", "ipv6", "add", "route", "8000::/1", name, "metric=1", "store=active"},
	}
	for _, a := range v6 {
		if err := runNetsh(a...); err != nil {
			return err
		}
	}
	return nil
}

func removeSplitRoutes(name string) error {
	var first error
	cmds := [][]string{
		{"interface", "ipv4", "delete", "route", "0.0.0.0/1", name},
		{"interface", "ipv4", "delete", "route", "128.0.0.0/1", name},
		{"interface", "ipv4", "delete", "route", "1.1.1.1/32", name},
		{"interface", "ipv6", "delete", "route", "::/1", name},
		{"interface", "ipv6", "delete", "route", "8000::/1", name},
	}
	for _, a := range cmds {
		if err := runNetsh(a...); err != nil && first == nil {
			first = err
		}
	}
	return first
}
