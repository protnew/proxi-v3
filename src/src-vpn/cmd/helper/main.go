package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
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
			for _, a := range os.Args[2:] {
				if a == "--hold" {
					hold = true
				}
			}
			if err := runConnectSmoke(hold); err != nil {
				fail(err)
			}
			os.Exit(0)
		case "--install":
			exe, err := os.Executable()
			if err != nil {
				fail(err)
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
				emit("ROLLBACK: helper.exe --killswitch-remove")
				emit("ROLLBACK: netsh wfp show filters")
				fail(err)
			}
			emit("killswitch-removed")
			os.Exit(0)
		case "--service":
			emit("helper-service")
			os.Exit(0)
		}
	}
	if os.Getenv("PROXI_HELPER_ELEVATED") != "1" {
		fmt.Fprintln(os.Stderr, "helper requires elevated spawn")
		os.Exit(2)
	}
	emit("helper-up")
}
