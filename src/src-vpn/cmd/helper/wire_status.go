package main

type wireStatus struct {
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

func wireFrom(state, phase, code, leg, errText, endpoint string, engaged, selfExit bool, attempt, max int) wireStatus {
	state, code = mapWireState(state, code)
	if state == "error" && code == "" {
		code = errText
		if code == "" {
			code = "unknown_state"
		}
	}
	out := wireStatus{
		State:     state,
		Phase:     phase,
		Code:      code,
		Leg:       leg,
		SelfExit:  selfExit,
		Engaged:   engaged,
		ActiveLeg: leg,
		Endpoint:  endpoint,
		Error:     errText,
		Attempt:   attempt,
		Max:       max,
	}
	if state != "connecting" {
		out.Phase = ""
	}
	if state != "reconnecting" {
		out.Attempt = 0
		out.Max = 0
	}
	if state != "error" {
		out.Code = ""
	}
	if state != "connected" {
		out.Leg = ""
		out.SelfExit = false
	}
	return out
}

func onPipeClientGone(clients int, engaged bool) (string, bool) {
	if clients > 0 || !engaged {
		return "", false
	}
	return "locked", true
}

type resumePlan struct {
	ReassertRoutes bool
	HealthCheck    bool
	SignalCore     bool
	Actions        []string
}

func planResume(engaged bool, adapter string) resumePlan {
	if !engaged {
		return resumePlan{}
	}
	return resumePlan{
		ReassertRoutes: true,
		HealthCheck:    adapter != "",
		SignalCore:     true,
		Actions:        []string{"reassert-routes", "adapter-health " + adapter, "signal-core"},
	}
}

func dhcpPermitOK(layer string, udp bool, port int) bool {
	connect := layer == "connect-v4" || layer == "connect-v6"
	if !connect || !udp {
		return false
	}
	if layer == "connect-v4" {
		return port == 67 || port == 68
	}
	return port == 546 || port == 547
}
