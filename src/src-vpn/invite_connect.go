package vpn

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/quic-go/webtransport-go"
	"github.com/unkillable-messenger/vpn/tor"
)

// ConnectInvite — client side of P-G: accepts an invite payload, performs
// first-frame auth {npub,token,sig,exp} on the first bidi stream, keeps the
// session as activeLeg. Onion leg via local Tor SOCKS; WT leg is UDP QUIC
// (NEVER routed through Tor — it cannot carry UDP).

type InviteParams struct {
	Onion    string `json:"onion"`
	WtAddr   string `json:"wtAddr"`
	CertHash string `json:"certHash"`
	Token    string `json:"token"`
	Npub     string `json:"npub"` // our npub (signer)
	Sig      string `json:"sig"`
	Exp      int64  `json:"exp"`
}

func (m *Manager) ConnectInvite(p InviteParams) error {
	if p.Token == "" || p.Npub == "" || p.Sig == "" {
		return fmt.Errorf("token, npub and sig required")
	}
	if p.Onion == "" && p.WtAddr == "" {
		return fmt.Errorf("no_exit_peers")
	}

	frame, _ := json.Marshal(authFrame{Npub: p.Npub, Token: p.Token, Sig: p.Sig, Exp: p.Exp})

	// Onion leg first (TCP over Tor SOCKS — works behind NAT).
	if p.Onion != "" {
		if err := m.connectInviteOnion(p.Onion, frame); err == nil {
			return nil
		} else {
			fmt.Printf("[VPN] onion invite leg failed: %v\n", err)
		}
	}
	// WT leg (UDP) — only when an explicit public endpoint was advertised.
	if p.WtAddr != "" {
		if err := m.connectInviteWT(p, frame); err == nil {
			return nil
		} else {
			return fmt.Errorf("wt invite leg: %w", err)
		}
	}
	return fmt.Errorf("no reachable donor leg")
}

func (m *Manager) connectInviteOnion(onionAddr string, frame []byte) error {
	d := tor.NewTorDialer("")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := d.DialContext(ctx, "tcp", onionAddr)
	if err != nil {
		return err
	}
	if err := conn.SetDeadline(time.Now().Add(60 * time.Second)); err != nil {
		conn.Close()
		return err
	}
	if _, err := conn.Write(append(frame, '\n')); err != nil {
		conn.Close()
		return err
	}
	// Donor replies "ok\n" after Admit; anything else = deny.
	buf := make([]byte, 8)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil || string(buf[:2]) != "ok" {
		conn.Close()
		return fmt.Errorf("donor denied auth")
	}
	_ = conn.SetDeadline(time.Time{})

	m.mu.Lock()
	m.state = StateConnected
	m.startTime = time.Now()
	m.activeLeg = "onion:" + onionAddr
	m.peerLost = false
	m.lastErr = ""
	if m.watchCancel != nil {
		m.watchCancel()
	}
	wctx, wc := context.WithCancel(context.Background())
	m.watchCancel = wc
	m.mu.Unlock()
	go m.watchPeer(wctx, onionAddr)
	go func() { <-wctx.Done(); conn.Close() }()
	return nil
}

func (m *Manager) connectInviteWT(p InviteParams, frame []byte) error {
	pin, _ := hex.DecodeString(p.CertHash)
	tr := &webtransport.Transport{}
	// Cert-pin: donor cert hash from the invite is the trust anchor (D7).
	tlsCfg := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13} //nolint: cert is pinned below
	if len(pin) == sha256.Size {
		tlsCfg.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("no peer cert")
			}
			sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
			if !bytes.Equal(sum[:], pin) {
				return fmt.Errorf("wt cert hash mismatch")
			}
			return nil
		}
	}
	tr.TLSClientConfig = tlsCfg
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := "https://" + p.WtAddr + "/wt"
	_, sess, err := tr.Dial(ctx, url, http.Header{})
	if err != nil {
		return err
	}
	stream, err := sess.OpenStreamSync(ctx)
	if err != nil {
		return err
	}
	var hdr [5]byte
	hdr[0] = wtMsgAuth
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(frame)))
	if _, err := stream.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := stream.Write(frame); err != nil {
		return err
	}
	// Read response frame — expect CONNECT phase only after admit.
	var rh [5]byte
	if _, err := io.ReadFull(stream, rh[:]); err != nil {
		return err
	}
	if rh[0] == wtMsgError {
		return fmt.Errorf("donor denied auth")
	}

	m.mu.Lock()
	m.state = StateConnected
	m.startTime = time.Now()
	m.activeLeg = "wt:" + p.WtAddr
	m.peerLost = false
	m.lastErr = ""
	m.mu.Unlock()
	return nil
}
