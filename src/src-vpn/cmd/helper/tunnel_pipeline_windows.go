//go:build windows

package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/tun"
)

const tunAdapter = "Proxi0"
const smokeExit = "203.0.113.9"

type tunnelOpt struct {
	Exit string
	Hold bool
}

type tunnelSession struct {
	dev      tun.Device
	name     string
	exitIP   string
	physIf   string
	physHop  string
	physIdx  uint32
	wfpOn    bool
	routesOn bool
	hostOn   bool
	peerOn   bool
	dnsStop  func()
	closed   bool
}

var (
	tunnelMu sync.Mutex
	liveSess *tunnelSession
)

func runTunnelPipeline(opt tunnelOpt) error {
	notePhase("service")
	exit := opt.Exit
	if exit == "" {
		exit = smokeExit
	}
	ip, err := preResolve(exit)
	if err != nil {
		return stepErr("no_exit_peers", err)
	}
	sess, err := openTunnel(ip)
	if err != nil {
		return err
	}
	defer sess.rollback()
	if opt.Hold {
		emit("hold")
		waitHoldStop()
	}
	return nil
}

func preResolve(host string) (string, error) {
	if host == "" {
		return "", fmt.Errorf("empty exit")
	}
	if ip := net.ParseIP(host); ip != nil {
		emit("preresolved " + ip.String())
		return ip.String(), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return "", err
	}
	emit("preresolved " + ips[0].String())
	return ips[0].String(), nil
}

func openTunnel(exitIP string) (*tunnelSession, error) {
	s := &tunnelSession{exitIP: exitIP, name: tunAdapter}
	alias, hop, idx, err := physicalDefault()
	if err != nil {
		return nil, stepErr("route_add_failed", err)
	}
	s.physIf, s.physHop, s.physIdx = alias, hop, idx
	emit(fmt.Sprintf("physical if=%s hop=%s idx=%d", alias, hop, idx))
	dev, err := tun.CreateTUN(tunAdapter, 1280)
	if err != nil {
		return nil, stepErr("wintun_missing", err)
	}
	s.dev = dev
	if n, nerr := dev.Name(); nerr == nil && n != "" {
		s.name = n
	}
	nt, ok := dev.(*tun.NativeTun)
	if !ok {
		s.rollback()
		return nil, stepErr("wintun_missing", fmt.Errorf("tun is not NativeTun"))
	}
	luid := uint64(nt.LUID())
	emit(fmt.Sprintf("adapter created LUID=%d name=%s", luid, s.name))
	notePhase("tun")
	if err = installKillSwitch(luid); err != nil {
		s.rollback()
		return nil, stepErr("access_denied", err)
	}
	s.wfpOn = true
	emit("wfp-engaged")
	if err = proveEngagedBlock(); err != nil {
		s.rollback()
		return nil, stepErr("access_denied", err)
	}
	if err = assignTunAddr(s.name); err != nil {
		s.rollback()
		return nil, stepErr("route_add_failed", err)
	}
	if err = setTunDNS(s.name); err != nil {
		s.rollback()
		return nil, stepErr("route_add_failed", err)
	}
	if err = addPeerRoute(s.name); err != nil {
		s.rollback()
		return nil, stepErr("route_add_failed", err)
	}
	s.peerOn = true
	if err = addHostRoute(exitIP, s.physIf, s.physHop); err != nil {
		s.rollback()
		return nil, stepErr("route_add_failed", err)
	}
	s.hostOn = true
	emit("host-route " + exitIP + "/32 via " + s.physHop)
	if err = installSplitRoutes(s.name); err != nil {
		s.rollback()
		return nil, stepErr("route_add_failed", err)
	}
	s.routesOn = true
	if s.routesOn && !s.wfpOn {
		s.rollback()
		return nil, stepErr("route_add_failed", fmt.Errorf("routes without wfp"))
	}
	notePhase("routes")
	emit("routes-installed")
	addr, stopStub, err := startLoopbackDNSStub()
	if err != nil {
		s.rollback()
		return nil, stepErr("connect_timeout", err)
	}
	stopRelay := startTunRelay(dev, addr)
	s.dnsStop = func() { stopRelay(); stopStub() }
	emit("dns-listen 10.7.0.1:53 tun-relay upstream=" + addr)
	notePhase("transport")
	if err = probeExit(exitIP, s.physIdx); err != nil {
		s.rollback()
		return nil, stepErr("access_denied", err)
	}
	notePhase("exit-probe")
	return s, nil
}

func (s *tunnelSession) rollback() {
	if s == nil || s.closed {
		return
	}
	s.closed = true
	if s.dnsStop != nil {
		s.dnsStop()
	}
	var routeErr error
	if s.routesOn {
		if err := removeSplitRoutes(s.name); err != nil {
			routeErr = err
		}
	}
	if s.hostOn {
		if err := delHostRoute(s.exitIP, s.physIf); err != nil && routeErr == nil {
			routeErr = err
		}
	}
	if s.peerOn {
		_ = delPeerRoute(s.name)
	}
	_ = clearTunDNS(s.name)
	if routeErr != nil {
		emit("ROLLBACK routes remain; WFP left engaged")
		emit("ROLLBACK: proxi04-vpn-helper.exe --killswitch-remove")
		printRollback()
		return
	}
	if s.wfpOn {
		if err := removeKillSwitch(); err != nil {
			emit("ROLLBACK: proxi04-vpn-helper.exe --killswitch-remove")
			printRollback()
			return
		}
		s.wfpOn = false
	}
	if s.dev != nil {
		_ = s.dev.Close()
	}
	if err := assertNoSplitRoutes(s.name); err != nil {
		emit("ERROR " + err.Error())
		return
	}
	emit("routes-clean")
}

func addHostRoute(ip, iface, hop string) error {
	return runNetsh("interface", "ipv4", "add", "route", ip+"/32", iface, hop, "metric=1", "store=active")
}

func delHostRoute(ip, iface string) error {
	return runNetsh("interface", "ipv4", "delete", "route", ip+"/32", iface)
}

func addPeerRoute(name string) error {
	return runNetsh("interface", "ipv4", "add", "route", "10.7.0.1/32", name, "0.0.0.0", "metric=1", "store=active")
}

func delPeerRoute(name string) error {
	return runNetsh("interface", "ipv4", "delete", "route", "10.7.0.1/32", name)
}

func probeExit(ip string, ifIndex uint32) error {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP(ip), Port: 51820})
	if err != nil {
		emit("exit-probe dial " + err.Error())
		return nil
	}
	defer conn.Close()
	if err = setUnicastIF(conn, ifIndex); err != nil {
		return fmt.Errorf("IP_UNICAST_IF: %w", err)
	}
	emit(fmt.Sprintf("exit-probe bound ifindex=%d proto=IP_UNICAST_IF", ifIndex))
	_ = conn.SetDeadline(time.Now().Add(300 * time.Millisecond))
	_, _ = conn.Write([]byte{0})
	return nil
}

func setUnicastIF(conn *net.UDPConn, ifIndex uint32) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		sockErr = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, 31, int(htonl(ifIndex)))
	})
	if err != nil {
		return err
	}
	return sockErr
}

func notePhase(phase string) {
	emit("phase=" + phase)
	if activeState == nil {
		return
	}
	activeState.mu.Lock()
	activeState.phase = phase
	activeState.state = "connecting"
	activeState.mu.Unlock()
}

func stepErr(code string, err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "wintun") || strings.Contains(msg, ".dll") {
		code = "wintun_missing"
	}
	emit("error code=" + code)
	if activeState != nil {
		activeState.mu.Lock()
		activeState.state = "error"
		activeState.lastErr = code
		activeState.code = code
		activeState.mu.Unlock()
	}
	return fmt.Errorf("%s: %w", code, err)
}

func startTunnelFromPipe(endpoint string, selfExit bool) {
	if endpoint == "" && selfExit {
		endpoint = smokeExit
	}
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	if liveSess != nil {
		return
	}
	sess, err := openTunnel(endpoint)
	if err != nil {
		emit("tunnel-error " + err.Error())
		return
	}
	liveSess = sess
}

func stopLiveTunnel() {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	if liveSess == nil {
		return
	}
	liveSess.rollback()
	liveSess = nil
}

func waitHoldStop() {
	exe, err := os.Executable()
	dir := "."
	if err == nil {
		dir = filepathDir(exe)
	}
	stop := dir + `\connect-smoke.stop`
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stop); err == nil {
			_ = os.Remove(stop)
			emit("hold-stop")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	emit("hold-timeout")
}

func filepathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
