// Helper stage 1 (D8=K): elevated spawn only. wintun/WFP land behind -tags helperdeps.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--self-test" {
		fmt.Println("elevated-spawn-ready")
		os.Exit(0)
	}
	if os.Getenv("PROXI_HELPER_ELEVATED") != "1" {
		fmt.Fprintln(os.Stderr, "helper requires elevated spawn")
		os.Exit(2)
	}
	fmt.Println("helper-up")
}
