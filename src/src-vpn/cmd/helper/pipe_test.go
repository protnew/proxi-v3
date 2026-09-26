package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPipe_StatusAndPrivateReject(t *testing.T) {
	var in, out bytes.Buffer
	in.WriteString("{\"verb\":\"status\"}\n")
	in.WriteString("{\"verb\":\"connect\",\"endpoint\":\"192.168.1.1:443\"}\n")
	in.WriteString("{\"verb\":\"connect\",\"endpoint\":\"203.0.113.9:443\"}\n")
	engaged := false
	servePipeConn(&in, &out, &engaged)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines %d", len(lines))
	}
	var st pipeStatus
	if err := json.Unmarshal([]byte(lines[0]), &st); err != nil || st.State != "off" || st.Engaged {
		t.Fatalf("status %+v %v", st, err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &st); err != nil || st.Error != "private_endpoint" {
		t.Fatalf("private %+v", st)
	}
	if err := json.Unmarshal([]byte(lines[2]), &st); err != nil || st.State != "connecting" || !engaged {
		t.Fatalf("public %+v engaged=%v", st, engaged)
	}
}
