package vpn

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type donorHold struct {
	Bind     string
	Token    string
	Exp      int64
	BindNpub string
	Onion    string
	WtAddr   string
	CertHash string
}

func donorRPC(m *Manager, method string, raw json.RawMessage, auth *ExitAuth) (interface{}, error) {
	var in struct {
		PublicBind string `json:"public_bind"`
		To         string `json:"to"`
		Token      string `json:"token"`
		Npub       string `json:"npub"`
		Sig        string `json:"sig"`
		Exp        int64  `json:"exp"`
		Onion      string `json:"onion"`
		WtAddr     string `json:"wtAddr"`
		CertHash   string `json:"certHash"`
	}
	_ = json.Unmarshal(raw, &in)
	switch method {
	case "start_egress_listener":
		// Onion leg is always available; public_bind only adds a WT leg.
		info, err := m.StartEgressListener(in.PublicBind)
		if err != nil {
			return nil, err
		}
		return info, nil
	case "stop_egress":
		m.StopEgress()
		return map[string]string{"state": "off"}, nil
	case "create_invite":
		if in.To == "" {
			return nil, fmt.Errorf("to required")
		}
		token, exp, err := issueDonorToken(auth, in.To)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"type": "vpn_invite", "from": "", "to": in.To,
			"token": token, "exp": exp, "ts": time.Now().Unix(), "v": 1,
		}, nil
	case "connect_invite":
		if in.Token == "" || in.Npub == "" {
			return nil, fmt.Errorf("token and npub required")
		}
		if in.Onion == "" && in.WtAddr == "" {
			return nil, fmt.Errorf("no_exit_peers")
		}
		if err := m.ConnectInvite(InviteParams{
			Onion: in.Onion, WtAddr: in.WtAddr, CertHash: in.CertHash,
			Token: in.Token, Npub: in.Npub,
			Sig: in.Sig, Exp: in.Exp,
		}); err != nil {
			return nil, err
		}
		m.mu.Lock()
		leg := m.activeLeg
		m.mu.Unlock()
		return map[string]string{"state": "accepted", "leg": leg}, nil
	default:
		return nil, fmt.Errorf("method not found")
	}
}

func issueDonorToken(auth *ExitAuth, bindNpub string) (string, int64, error) {
	if auth == nil {
		return "", 0, fmt.Errorf("exit auth missing")
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", 0, err
	}
	token := hex.EncodeToString(buf)
	exp := time.Now().Unix() + 15*60
	if err := auth.Issue(token, exp, bindNpub); err != nil {
		return "", 0, err
	}
	return token, exp, nil
}
