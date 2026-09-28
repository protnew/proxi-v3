//go:build windows

package main

import (
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

func startTunRelay(dev tun.Device, upstream string) func() {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		buf := make([]byte, 2048)
		sizes := []int{0}
		for {
			select {
			case <-done:
				return
			default:
			}
			_, err := dev.Read([][]byte{buf}, sizes, 0)
			if err != nil || sizes[0] == 0 {
				return
			}
			pkt := append([]byte(nil), buf[:sizes[0]]...)
			resp, rerr := relayDNSPacket(pkt, upstream)
			if rerr != nil || resp == nil {
				continue
			}
			_, _ = dev.Write([][]byte{resp}, 0)
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}
