//go:build windows

package main

import (
	"fmt"
	"net"
	"time"
)

func proveEngagedBlock() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	loopOK := dialOK(ln.Addr().String(), 2*time.Second)
	unreachableFail := !dialOK("203.0.113.9:9", 1200*time.Millisecond)
	emit(fmt.Sprintf("wfp-selfcheck loopback=%t unreachable-fail=%t", loopOK, unreachableFail))
	return classifyBlockProbe(loopOK, unreachableFail)
}

func dialOK(addr string, wait time.Duration) bool {
	c, err := net.DialTimeout("tcp", addr, wait)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
