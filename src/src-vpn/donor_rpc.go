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

func donorRPC(method string, raw json.RawMessage) (interface{}, error) {
	var in struct {
		PublicBind string `json:"public_bind"`
		To         string `json:"to"`
		Token      string `json:"token"`
		Npub       string `json:"npub"`
		Exp        int64  `json:"exp"`
		Onion      string `json:"onion"`
		WtAddr     string `json:"wtAddr"`
		CertHash   string `json:"certHash"`
	}
	_ = json.Unmarshal(raw, &in)
	switch method {
	case "start_egress_listener":
		if in.PublicBind == "" {
			return nil, fmt.Errorf("public_bind required")
		}
		return map[string]string{
			"state":    "stub",
			"wtAddr":   "",
			"onion":    "",
			"certHash": "",
			"reason":   "listener_not_in_this_build",
		}, nil
	case "stop_egress":
		return map[string]string{"state": "off"}, nil
	case "create_invite":
		if in.To == "" {
			return nil, fmt.Errorf("to required")
		}
		token, exp, err := issueDonorToken(in.To)
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
		return map[string]string{"state": "accepted", "code": "handshake_not_in_this_build"}, nil
	default:
		return nil, fmt.Errorf("method not found")
	}
}

func issueDonorToken(bindNpub string) (string, int64, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", 0, err
	}
	exp := time.Now().Unix() + 15*60
	return hex.EncodeToString(buf), exp, nil
}
