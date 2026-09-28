package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--self-test":
			fmt.Println("elevated-spawn-ready")
			os.Exit(0)
		case "--netstack":
			if _, err := startNetstack(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Println("netstack-up")
			os.Exit(0)
		case "--install":
			exe, err := os.Executable()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if err := installHelperService(exe); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Println("service-installed")
			os.Exit(0)
		case "--killswitch":
			if err := installKillSwitch(0); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Println("killswitch-installed")
			os.Exit(0)
		case "--service":
			fmt.Println("helper-service")
			os.Exit(0)
		}
	}
	if os.Getenv("PROXI_HELPER_ELEVATED") != "1" {
		fmt.Fprintln(os.Stderr, "helper requires elevated spawn")
		os.Exit(2)
	}
	fmt.Println("helper-up")
}
