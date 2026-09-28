package vpn

import (
	"encoding/json"
	"fmt"
	"io"
)

var dialHelperPipe func() (io.ReadWriteCloser, error)

// queryHelperStatus is a best-effort pipe status read (nil-safe).
func queryHelperStatus() (HelperStatus, error) {
	if dialHelperPipe == nil {
		return HelperStatus{}, fmt.Errorf("helper_unavailable")
	}
	rw, err := dialHelperPipe()
	if err != nil {
		return HelperStatus{}, err
	}
	defer rw.Close()
	return QueryHelper(rw)
}

// helperVerbViaPipe sends disconnect/disarm/unlock to the helper.
func helperVerbViaPipe(verb string) (any, error) {
	if dialHelperPipe == nil {
		return nil, fmt.Errorf("helper_unavailable")
	}
	rw, err := dialHelperPipe()
	if err != nil {
		return nil, err
	}
	defer rw.Close()
	st, err := HelperVerb(rw, verb)
	if err != nil {
		return nil, err
	}
	return map[string]any{"state": st.State, "engaged": st.Engaged, "code": st.Code, "error": st.Error}, nil
}

func connectExitViaPipe(params json.RawMessage) (any, error) {
	var p struct {
		Endpoint   string `json:"endpoint"`
		Onion      string `json:"onion"`
		WtCertHash string `json:"wtCertHash"`
		Token      string `json:"token"`
		Npub       string `json:"npub"`
		Sig        string `json:"sig"`
		Exp        int64  `json:"exp"`
		SelfExit   bool   `json:"self_exit"`
	}
	if len(params) > 0 && string(params) != "null" {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
	}
	if dialHelperPipe == nil {
		return nil, fmt.Errorf("helper_unavailable")
	}
	rw, err := dialHelperPipe()
	if err != nil {
		return nil, err
	}
	defer rw.Close()
	st, err := ConnectHelper(rw, HelperConnect{
		Endpoint:   p.Endpoint,
		Onion:      p.Onion,
		WtCertHash: p.WtCertHash,
		Token:      p.Token,
		Npub:       p.Npub,
		Sig:        p.Sig,
		Exp:        p.Exp,
		SelfExit:   p.SelfExit,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"state": st.State, "engaged": st.Engaged, "error": st.Error}, nil
}
