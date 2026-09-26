//go:build !windows

package main

import (
	"errors"
	"net"
)

func clientImageFromConn(net.Conn) (string, error) {
	return "", errors.New("pipe client image is windows-only")
}

func installedCorePath() string { return "" }

func rememberCorePath(string) error { return nil }
