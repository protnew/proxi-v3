//go:build windows

package main

import (
	"net"

	"golang.org/x/sys/windows/svc"
)

type helperSvc struct{}

func (helperSvc) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	ln, err := listenHelperPipe()
	if err != nil {
		return false, 1
	}
	defer ln.Close()
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPowerEvent | svc.AcceptSessionChange}
	st := newHelperState()
	go acceptPipe(ln, st)
	for c := range r {
		switch c.Cmd {
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		case svc.PowerEvent:
			if handlePowerEvent(uint32(c.Cmd), c.EventType).Reassert {
				powerEvent()
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return false, 0
}

func acceptPipe(ln net.Listener, st *helperState) {
	activeState = st
	engageTunnel = startTunnelFromPipe
	releaseTunnel = stopLiveTunnel
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			defer conn.Close()
			img, _ := clientImageFromConn(conn)
			servePipeConn(conn, conn, st, img)
		}(c)
	}
}

func runHelperService() error {
	is, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !is {
		return runPipeForeground()
	}
	return svc.Run(helperServiceName, helperSvc{})
}

func runPipeForeground() error {
	ln, err := listenHelperPipe()
	if err != nil {
		return err
	}
	defer ln.Close()
	emit("pipe-listening")
	acceptPipe(ln, newHelperState())
	return nil
}
