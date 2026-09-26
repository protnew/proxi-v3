package main

import (
	"bufio"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"sync"
)

const pipeName = `\\.\pipe\ProxiHelper`

type pipeCmd struct {
	Verb       string `json:"verb"`
	Endpoint   string `json:"endpoint,omitempty"`
	Onion      string `json:"onion,omitempty"`
	WtCertHash string `json:"wtCertHash,omitempty"`
	Token      string `json:"token,omitempty"`
	Npub       string `json:"npub,omitempty"`
	Sig        string `json:"sig,omitempty"`
	Exp        int64  `json:"exp,omitempty"`
	SelfExit   bool   `json:"self_exit,omitempty"`
	ClientExe  string `json:"client_exe,omitempty"`
}

type pipeStatus struct {
	State     string `json:"state"`
	Phase     string `json:"phase,omitempty"`
	Attempt   int    `json:"attempt,omitempty"`
	Max       int    `json:"max,omitempty"`
	Code      string `json:"code,omitempty"`
	Leg       string `json:"leg,omitempty"`
	SelfExit  bool   `json:"selfExit,omitempty"`
	Engaged   bool   `json:"engaged"`
	ActiveLeg string `json:"active_leg,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Error     string `json:"error,omitempty"`
}

type helperState struct {
	mu         sync.Mutex
	state      string
	phase      string
	code       string
	engaged    bool
	activeLeg  string
	endpoint   string
	lastErr    string
	corePath   string
	token      string
	npub       string
	sig        string
	exp        int64
	onion      string
	wtCertHash string
	selfExit   bool
	attempt    int
	attemptMax int
	clients    int
}

func newHelperState() *helperState {
	return &helperState{state: "off"}
}

func (s *helperState) expectedCore() string {
	if s.corePath != "" {
		return s.corePath
	}
	return installedCorePath()
}

func (s *helperState) controlAllowed(clientImage string) bool {
	want := s.expectedCore()
	if want == "" || clientImage == "" {
		return false
	}
	return sameExe(clientImage, want)
}

func (s *helperState) denyForeign() pipeStatus {
	snap := s.snapshot()
	snap.State = "error"
	snap.Error = "client_exe_denied"
	return snap
}

func (s *helperState) apply(cmd pipeCmd, clientImage string) pipeStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch cmd.Verb {
	case "status":
		return s.snapshot()
	case "connect", "disconnect", "disarm", "unlock":
		if !s.controlAllowed(clientImage) {
			return s.denyForeign()
		}
	default:
		s.lastErr = "unknown_verb"
		s.state = "error"
		return s.snapshot()
	}
	switch cmd.Verb {
	case "connect":
		if cmd.Endpoint == "" && cmd.Onion == "" && !cmd.SelfExit {
			s.lastErr = "no_exit_peers"
			s.state = "error"
			return s.snapshot()
		}
		if !cmd.SelfExit && cmd.Endpoint != "" && privateEndpoint(cmd.Endpoint) {
			s.lastErr = "private_endpoint"
			s.state = "error"
			return s.snapshot()
		}
		s.engaged = true
		s.state = "connecting"
		s.phase = "service"
		s.code = ""
		s.activeLeg = "userspace"
		s.endpoint = cmd.Endpoint
		s.onion = cmd.Onion
		s.wtCertHash = cmd.WtCertHash
		s.token = cmd.Token
		s.npub = cmd.Npub
		s.sig = cmd.Sig
		s.exp = cmd.Exp
		s.selfExit = cmd.SelfExit
		s.attempt = 0
		s.attemptMax = 0
		if cmd.Onion != "" {
			s.activeLeg = "tor"
			s.endpoint = cmd.Onion
		}
		s.lastErr = ""
		return s.snapshot()
	case "disconnect", "disarm", "unlock":
		s.engaged = false
		s.state = "off"
		s.phase = ""
		s.code = ""
		s.selfExit = false
		s.attempt = 0
		s.attemptMax = 0
		s.activeLeg = ""
		s.endpoint = ""
		s.token = ""
		s.npub = ""
		s.sig = ""
		s.exp = 0
		s.lastErr = ""
		return s.snapshot()
	default:
		s.lastErr = "unknown_verb"
		s.state = "error"
		return s.snapshot()
	}
}

func (s *helperState) snapshot() pipeStatus {
	state, code := mapWireState(s.state, s.code)
	if state == "error" && code == "" {
		code = s.lastErr
	}
	wire := wireFrom(state, s.phase, code, s.activeLeg, s.lastErr, s.endpoint, s.engaged, s.selfExit, s.attempt, s.attemptMax)
	return pipeStatus{
		State:     wire.State,
		Phase:     wire.Phase,
		Attempt:   wire.Attempt,
		Max:       wire.Max,
		Code:      wire.Code,
		Leg:       wire.Leg,
		SelfExit:  wire.SelfExit,
		Engaged:   wire.Engaged,
		ActiveLeg: s.activeLeg,
		Endpoint:  s.endpoint,
		Error:     s.lastErr,
	}
}

func (s *helperState) clientClosed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clients > 0 {
		s.clients--
	}
	state, hold := onPipeClientGone(s.clients, s.engaged)
	if !hold {
		return
	}
	s.state = state
	s.lastErr = ""
	emit("pipe-client-gone hold engaged state=locked")
}

func servePipeConn(r io.Reader, w io.Writer, st *helperState, clientImage string) {
	sc := bufio.NewScanner(r)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		var cmd pipeCmd
		line := strings.TrimSpace(sc.Text())
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			_ = enc.Encode(pipeStatus{State: "error", Error: "bad_json"})
			continue
		}
		status := st.apply(cmd, clientImage)
		if cmd.Verb == "connect" && status.State == "connecting" && engageTunnel != nil {
			st.mu.Lock()
			st.clients++
			st.mu.Unlock()
			go engageTunnel(cmd.Endpoint, cmd.SelfExit)
		}
		if cmd.Verb == "disconnect" && releaseTunnel != nil {
			releaseTunnel()
			status = st.apply(pipeCmd{Verb: "status"}, clientImage)
		}
		_ = enc.Encode(status)
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

func sameExe(a, b string) bool {
	a = strings.TrimPrefix(strings.TrimSpace(a), `\\?\`)
	b = strings.TrimPrefix(strings.TrimSpace(b), `\\?\`)
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
