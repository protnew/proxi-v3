//go:build !windows

package main

import (
	"errors"
	"net"
)

func listenHelperPipe() (net.Listener, error) {
	return nil, errors.New("named pipe is windows-only")
}
