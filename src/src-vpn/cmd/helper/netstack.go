package main

import (
	"fmt"
	"net/netip"

	"golang.zx2c4.com/wireguard/tun/netstack"
)

func startNetstack() (string, error) {
	dev, net, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("10.7.0.2")},
		[]netip.Addr{netip.MustParseAddr("10.7.0.1")},
		1280,
	)
	if err != nil {
		return "", err
	}
	defer dev.Close()
	if net == nil {
		return "", fmt.Errorf("gvisor netstack nil")
	}
	return "gvisor-netstack", nil
}
