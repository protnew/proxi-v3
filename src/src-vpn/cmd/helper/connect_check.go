package main

import (
	"fmt"
	"net"
	"time"
)

func smokeSelfCheck() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			_ = c.Close()
		}
	}()
	d := net.Dialer{Timeout: 800 * time.Millisecond}
	ok, err := d.Dial("tcp", ln.Addr().String())
	if err != nil {
		return fmt.Errorf("loopback permit failed: %w", err)
	}
	_ = ok.Close()

	ext, err := d.Dial("tcp", "1.1.1.1:443")
	if err == nil {
		_ = ext.Close()
		return fmt.Errorf("external tcp succeeded while WFP engaged")
	}
	return nil
}
