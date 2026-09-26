package main

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

const pipeName = `\\.\pipe\ProxiHelper`

type pipeCmd struct {
	Verb     string `json:"verb"`
	Endpoint string `json:"endpoint,omitempty"`
	SelfExit bool   `json:"self_exit,omitempty"`
}

type pipeStatus struct {
	State   string `json:"state"`
	Engaged bool   `json:"engaged"`
	Error   string `json:"error,omitempty"`
}

func handlePipeLine(line string, engaged bool) pipeStatus {
	var cmd pipeCmd
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &cmd); err != nil {
		return pipeStatus{State: "error", Error: "bad_json"}
	}
	switch cmd.Verb {
	case "status":
		if engaged {
			return pipeStatus{State: "connected", Engaged: true}
		}
		return pipeStatus{State: "off", Engaged: false}
	case "connect":
		if cmd.Endpoint == "" && !cmd.SelfExit {
			return pipeStatus{State: "error", Error: "no_exit_peers"}
		}
		if !cmd.SelfExit && privateEndpoint(cmd.Endpoint) {
			return pipeStatus{State: "error", Error: "private_endpoint"}
		}
		return pipeStatus{State: "connecting", Engaged: true}
	case "disconnect", "disarm", "unlock":
		return pipeStatus{State: "off", Engaged: false}
	default:
		return pipeStatus{State: "error", Error: "unknown_verb"}
	}
}

func servePipeConn(r io.Reader, w io.Writer, engaged *bool) {
	sc := bufio.NewScanner(r)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		st := handlePipeLine(sc.Text(), *engaged)
		if st.Error == "" && (st.State == "connecting" || st.State == "connected") {
			*engaged = true
		}
		if st.State == "off" {
			*engaged = false
		}
		_ = enc.Encode(st)
	}
}

func privateEndpoint(ep string) bool {
	host := ep
	if i := strings.LastIndex(ep, ":"); i > 0 {
		host = ep[:i]
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "169.254.")
}
