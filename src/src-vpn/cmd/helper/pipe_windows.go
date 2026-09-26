//go:build windows

package main

import (
	"net"

	"github.com/Microsoft/go-winio"
)

func listenHelperPipe() (net.Listener, error) {
	cfg := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;SY)(A;;GA;;;IU)",
		InputBufferSize:    4096,
		OutputBufferSize:   4096,
	}
	return winio.ListenPipe(pipeName, cfg)
}
