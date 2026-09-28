package main

import "fmt"

var connectPhases = []string{"service", "tun", "routes", "transport", "exit-probe"}

func mapWireState(state, code string) (string, string) {
	switch state {
	case "off", "connecting", "connected", "sharing", "reconnecting", "locked", "disconnecting", "error":
		if state == "error" && code == "" {
			return state, "unknown_state"
		}
		return state, code
	default:
		return "error", "unknown_state"
	}
}

func parsePhysLine(line string) (alias, hop string, index uint32, ok bool) {
	parts := splitPipe(line)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[1] == "0.0.0.0" {
		return "", "", 0, false
	}
	n := uint32(0)
	for _, c := range parts[2] {
		if c < '0' || c > '9' {
			return "", "", 0, false
		}
		n = n*10 + uint32(c-'0')
	}
	return parts[0], parts[1], n, true
}

func htonl(v uint32) uint32 {
	return v<<24 | (v&0xff00)<<8 | (v&0xff0000)>>8 | v>>24
}

func classifyBlockProbe(loopOK, externalBlocked bool) error {
	if !loopOK {
		return fmt.Errorf("loopback permit failed")
	}
	if !externalBlocked {
		return fmt.Errorf("external tcp was not blocked")
	}
	return nil
}

func splitPipe(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '|' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}
