package vpn

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/unkillable-messenger/vpn/tor"
)

// Donor egress listener (P-G): onion TCP listener is mandatory (NAT path);
// WT listener additionally only when public_bind is explicitly set.
// First frame on every inbound connection MUST be auth {npub,token,sig,exp}
// within 3s; deny closes the conn and is audit-logged (npub truncated only).

const (
	authFrameTimeout = 3 * time.Second
	authMaxAttempts  = 3
	egressSessionTTL = 30 * time.Minute
)

type authFrame struct {
	Npub  string `json:"npub"`
	Token string `json:"token"`
	Sig   string `json:"sig"`
	Exp   int64  `json:"exp"`
}

type egressState struct {
	wt        *WTServer
	wtAddr    string
	certHash  string
	onionMgr  *tor.AutoOnionManager
	onionAddr string
	ln        net.Listener
	cancel    context.CancelFunc
}

// auditDonor writes an audit line with truncated npub (no token/sig ever).
func auditDonor(format string, npub string, args ...any) {
	short := npub
	if len(short) > 8 {
		short = short[:8] + "…"
	}
	log.Printf("[DONOR] npub=%s %s", short, fmt.Sprintf(format, args...))
}

// admitFrame validates the first-frame auth of one inbound connection.
func admitFrame(auth *ExitAuth, f authFrame) error {
	if f.Npub == "" || f.Token == "" || f.Sig == "" {
		return fmt.Errorf("auth frame incomplete")
	}
	return auth.Admit(f.Npub, f.Token, f.Exp, f.Sig, time.Now().Unix())
}

// serveOnionConn: auth line -> "CONNECT host:port" line -> pinned TCP relay.
func (m *Manager) serveOnionConn(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(authFrameTimeout))
	br := bufio.NewReader(c)
	line, err := br.ReadString('\n')
	if err != nil {
		return
	}
	var f authFrame
	if json.Unmarshal([]byte(line), &f) != nil || admitFrame(m.exitAuth(), f) != nil {
		auditDonor("deny (auth)", f.Npub)
		return
	}
	_ = c.SetDeadline(time.Now().Add(egressSessionTTL))
	auditDonor("admit", f.Npub)
	_, _ = io.WriteString(c, "ok\n")

	req, err := br.ReadString('\n')
	if err != nil {
		return
	}
	var target string
	if _, err := fmt.Sscanf(req, "CONNECT %s", &target); err != nil || target == "" {
		return
	}
	pinned, denied := aclResolveTarget(target)
	if denied {
		auditDonor("acl-deny %s", f.Npub, target)
		_, _ = io.WriteString(c, "denied\n")
		return
	}
	up, err := net.DialTimeout("tcp", pinnedTarget(target, pinned), 10*time.Second)
	if err != nil {
		_, _ = io.WriteString(c, "dial-failed\n")
		return
	}
	defer up.Close()
	_, _ = io.WriteString(c, "ok\n")
	auditDonor("connect %s", f.Npub, target)

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(up, br); done <- struct{}{} }()
	go func() { _, _ = io.Copy(c, up); done <- struct{}{} }()
	<-done
	<-done
}

// StartEgressListener starts donor egress: onion listener always,
// WT listener only when publicBind is explicitly set (white-IP donors).
func (m *Manager) StartEgressListener(publicBind string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.egress != nil {
		return m.egressInfo(), nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	st := &egressState{cancel: cancel}

	// Local CONNECT-relay the onion service forwards to.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("egress relay listen: %w", err)
	}
	st.ln = ln
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go m.serveOnionConn(c)
		}
	}()

	// Onion service via Tor control (async retry when tor is absent).
	port := ln.Addr().(*net.TCPAddr).Port
	st.onionMgr = tor.NewAutoOnionManager("127.0.0.1:9051", port, m.config.DataDir)
	if err := st.onionMgr.Start(); err != nil {
		log.Printf("[DONOR] onion start deferred: %v", err)
	}
	st.onionAddr = st.onionMgr.GetOnionAddress()

	// WT leg — only on explicit public bind (never over Tor: QUIC=UDP).
	if publicBind != "" {
		wt, err := NewWTServer(publicBind)
		if err != nil {
			log.Printf("[DONOR] wt init: %v", err)
		} else {
			wt.Auth = m.exitAuth()
			if err := wt.Start(); err != nil {
				log.Printf("[DONOR] wt listen %s: %v", publicBind, err)
			} else {
				st.wt = wt
				st.wtAddr = wt.LocalAddr()
				st.certHash = wt.GetCertHash()
			}
		}
	}

	m.egress = st
	_ = ctx
	return m.egressInfo(), nil
}

func (m *Manager) egressInfo() map[string]string {
	e := m.egress
	onion := e.onionAddr
	if onion == "" && e.onionMgr != nil {
		onion = e.onionMgr.GetOnionAddress()
	}
	return map[string]string{
		"state":    "egress-on",
		"wtAddr":   e.wtAddr,
		"onion":    onion,
		"certHash": e.certHash,
	}
}

// StopEgress tears down the donor egress.
func (m *Manager) StopEgress() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.egress == nil {
		return
	}
	m.egress.cancel()
	if m.egress.ln != nil {
		_ = m.egress.ln.Close()
	}
	if m.egress.wt != nil {
		m.egress.wt.Stop()
	}
	if m.egress.onionMgr != nil {
		m.egress.onionMgr.Stop()
	}
	m.egress = nil
}
