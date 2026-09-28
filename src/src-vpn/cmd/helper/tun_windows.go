//go:build windows

package main

import (
	"fmt"

	"golang.zx2c4.com/wireguard/tun"
)

func startWintun(name string, mtu int) error {
	dev, err := tun.CreateTUN(name, mtu)
	if err != nil {
		return err
	}
	return dev.Close()
}

func smokeTun(name string) error {
	dev, err := tun.CreateTUN(name, 1280)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = dev.Close()
		}
	}()
	nt, ok := dev.(*tun.NativeTun)
	if !ok {
		return fmt.Errorf("tun is not NativeTun")
	}
	adapterName, err := dev.Name()
	if err != nil || adapterName == "" {
		adapterName = name
	}
	emit(fmt.Sprintf("adapter created LUID=%d name=%s", nt.LUID(), adapterName))
	if err := dev.Close(); err != nil {
		return err
	}
	closed = true
	emit("closed")
	return nil
}
