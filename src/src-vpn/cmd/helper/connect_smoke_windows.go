//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.zx2c4.com/wireguard/tun"
)

func runConnectSmoke(hold bool) error {
	dev, err := tun.CreateTUN("ProxiSmoke", 1280)
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
	name, err := dev.Name()
	if err != nil || name == "" {
		name = "ProxiSmoke"
	}
	luid := uint64(nt.LUID())
	emit(fmt.Sprintf("adapter created LUID=%d name=%s", luid, name))

	if err := installKillSwitch(luid); err != nil {
		return err
	}
	wfpOn := true
	defer func() {
		if wfpOn {
			if err := removeKillSwitch(); err != nil {
				emit("ROLLBACK: helper.exe --killswitch-remove")
				printRollback()
			}
		}
	}()
	emit("wfp-engaged")
	if err := smokeSelfCheck(); err != nil {
		return err
	}
	emit("self-check-ok")
	if err := assignTunAddr(name); err != nil {
		return err
	}
	if err := setTunDNS(name); err != nil {
		return err
	}
	if err := installSplitRoutes(name); err != nil {
		return err
	}
	emit("routes-installed")
	if hold {
		emit("hold")
		waitHoldStop()
	}
	if err := removeSplitRoutes(name); err != nil {
		emit("route-remove: " + err.Error())
	}
	_ = clearTunDNS(name)
	if err := removeKillSwitch(); err != nil {
		emit("ROLLBACK: helper.exe --killswitch-remove")
		printRollback()
		return err
	}
	wfpOn = false
	if err := dev.Close(); err != nil {
		return err
	}
	closed = true
	emit("routes-clean")
	return nil
}

func waitHoldStop() {
	exe, err := os.Executable()
	dir := "."
	if err == nil {
		dir = filepath.Dir(exe)
	}
	stop := filepath.Join(dir, "connect-smoke.stop")
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stop); err == nil {
			_ = os.Remove(stop)
			emit("hold-stop")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	emit("hold-timeout")
}
