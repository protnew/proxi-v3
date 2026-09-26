package vpn

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

type HelperStatus struct {
	State   string `json:"state"`
	Engaged bool   `json:"engaged"`
	Error   string `json:"error,omitempty"`
}

type HelperConnect struct {
	Endpoint   string
	Onion      string
	WtCertHash string
	Token      string
	Npub       string
	Sig        string
	Exp        int64
	SelfExit   bool
}

func QueryHelper(rw io.ReadWriter) (HelperStatus, error) {
	if _, err := io.WriteString(rw, "{\"verb\":\"status\"}\n"); err != nil {
		return HelperStatus{}, err
	}
	return readHelperStatus(rw)
}

func ConnectHelper(rw io.ReadWriter, c HelperConnect) (HelperStatus, error) {
	body, err := json.Marshal(map[string]any{
		"verb":       "connect",
		"endpoint":   c.Endpoint,
		"onion":      c.Onion,
		"wtCertHash": c.WtCertHash,
		"token":      c.Token,
		"npub":       c.Npub,
		"sig":        c.Sig,
		"exp":        c.Exp,
		"self_exit":  c.SelfExit,
	})
	if err != nil {
		return HelperStatus{}, err
	}
	if _, err := rw.Write(append(body, '\n')); err != nil {
		return HelperStatus{}, err
	}
	return readHelperStatus(rw)
}

func readHelperStatus(rw io.Reader) (HelperStatus, error) {
	line, err := bufio.NewReader(rw).ReadString('\n')
	if err != nil {
		return HelperStatus{}, err
	}
	var st HelperStatus
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &st); err != nil {
		return HelperStatus{}, err
	}
	return st, nil
}
