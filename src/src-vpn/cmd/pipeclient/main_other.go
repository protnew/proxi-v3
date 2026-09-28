//go:build !windows

// CI (ubuntu) must still compile this package: the named-pipe client is a
// Windows-only diagnostic tool.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "pipeclient is Windows-only (named pipe \\.\\pipe\\Proxi04VpnHelper)")
	os.Exit(1)
}
