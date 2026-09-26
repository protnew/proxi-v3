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

func QueryHelper(rw io.ReadWriter) (HelperStatus, error) {
	if _, err := io.WriteString(rw, "{\"verb\":\"status\"}\n"); err != nil {
		return HelperStatus{}, err
	}
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
