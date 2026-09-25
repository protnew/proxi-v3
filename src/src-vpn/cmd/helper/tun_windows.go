//go:build windows

package main

import "golang.zx2c4.com/wireguard/tun"

func startWintun(name string, mtu int) error {
	dev, err := tun.CreateTUN(name, mtu)
	if err != nil {
		return err
	}
	return dev.Close()
}
