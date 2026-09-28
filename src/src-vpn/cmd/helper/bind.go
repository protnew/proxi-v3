package main

import "github.com/unkillable-messenger/vpn"

func BindKillSwitch(ks *vpn.KillSwitch) {
	if ks == nil {
		return
	}
	ks.Executor = func(rule string) error {
		return installKillSwitch(0)
	}
}
