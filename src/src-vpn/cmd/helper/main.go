package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if !tokenElevated() && !allowUnelevated() {
		fmt.Fprintln(os.Stderr, "helper requires an elevated token")
		os.Exit(2)
	}
	for i, a := range os.Args {
		if a == "--log" && i+1 < len(os.Args) {
			helperLogPath = os.Args[i+1]
		}
		if strings.HasPrefix(a, "--log=") {
			helperLogPath = strings.TrimPrefix(a, "--log=")
		}
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--self-test":
			emit("elevated-spawn-ready")
			os.Exit(0)
		case "--netstack":
			if _, err := startNetstack(); err != nil {
				fail(err)
			}
			emit("netstack-up")
			os.Exit(0)
		case "--tun-smoke":
			if err := smokeTun("ProxiSmoke"); err != nil {
				fail(err)
			}
			os.Exit(0)
		case "--connect-smoke":
			hold := false
			noRoute := false
			adapterOnly := false
			for _, a := range os.Args[2:] {
				switch a {
				case "--hold":
					hold = true
				case "--smoke-no-route":
					noRoute = true
				case "--smoke-adapter-only":
					adapterOnly = true
					noRoute = true
				}
			}
			if err := runConnectSmoke(hold, noRoute, adapterOnly); err != nil {
				fail(err)
			}
			os.Exit(0)
		case "--selftest-acl":
			if err := selftestACL(); err != nil {
				fail(err)
			}
			os.Exit(0)
		case "--install":
			exe, err := os.Executable()
			if err != nil {
				fail(err)
			}
			roots := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")}
			if !pathUnderAdminDir(exe, roots) {
				fail(fmt.Errorf("refusing install: helper is not under Program Files"))
			}
			if err := installHelperService(exe); err != nil {
				fail(err)
			}
			emit("service-installed")
			os.Exit(0)
		case "--uninstall":
			if err := uninstallHelperService(); err != nil {
				fail(err)
			}
			emit("service-removed")
			os.Exit(0)
		case "--killswitch":
			fail(fmt.Errorf("refusing persistent install; use --killswitch-test"))
		case "--killswitch-test":
			if err := runKillSwitchTest(); err != nil {
				fail(err)
			}
			os.Exit(0)
		case "--killswitch-remove":
			if err := removeKillSwitch(); err != nil {
				emit("ROLLBACK: proxi04-vpn-helper.exe --killswitch-remove")
				emit("ROLLBACK: netsh wfp show filters")
				fail(err)
			}
			emit("killswitch-removed")
			os.Exit(0)
		case "--service":
			if err := runHelperService(); err != nil {
				fail(err)
			}
			os.Exit(0)
		case "--elevated-spawn":
			emit("elevated-spawn-ready")
			os.Exit(0)
		case "--diag":
			if _, err := runDiag(); err != nil {
				fail(err)
			}
			os.Exit(0)
		case "--disconnect":
			if err := removeSplitRoutes(tunAdapter); err != nil {
				emit("disconnect-routes-failed " + err.Error())
				os.Exit(1)
			}
			if err := removeKillSwitch(); err != nil {
				emit("disconnect-wfp-failed " + err.Error())
				os.Exit(1)
			}
			emit("disconnect-clean")
			os.Exit(0)
		case "--installer":
			dest := `C:\Program Files\Proxi`
			tor := ""
			for i := 1; i < len(os.Args)-1; i++ {
				if os.Args[i] == "--dest" {
					dest = os.Args[i+1]
				}
				if os.Args[i] == "--tor" {
					tor = os.Args[i+1]
				}
			}
			if err := runInstaller(dest, tor, []string{`C:\Program Files`, `C:\Program Files (x86)`}); err != nil {
				emit("installer-failed " + err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	emit("helper-up")
}

func allowUnelevated() bool {
	if len(os.Args) < 2 {
		return false
	}
	switch os.Args[1] {
	case "--self-test", "--netstack", "--diag":
		return true
	}
	return false
}
