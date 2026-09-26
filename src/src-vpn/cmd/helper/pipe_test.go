package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPipe_SecondConnSeesConnect(t *testing.T) {
	st := newHelperState()
	var in1, out1 bytes.Buffer
	in1.WriteString("{\"verb\":\"connect\",\"endpoint\":\"203.0.113.9:443\"}\n")
	servePipeConn(&in1, &out1, st)

	var in2, out2 bytes.Buffer
	in2.WriteString("{\"verb\":\"status\"}\n")
	servePipeConn(&in2, &out2, st)

	var got pipeStatus
	if err := json.Unmarshal([]byte(strings.TrimSpace(out2.String())), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != "connecting" || !got.Engaged || got.Endpoint != "203.0.113.9:443" {
		t.Fatalf("second conn lost state: %+v", got)
	}
}

func TestPipe_PrivateAndForeignUnlock(t *testing.T) {
	st := newHelperState()
	st.coreExe = `C:\Program Files\Proxi\core.exe`
	var in, out bytes.Buffer
	in.WriteString("{\"verb\":\"connect\",\"endpoint\":\"192.168.1.1:443\"}\n")
	in.WriteString("{\"verb\":\"disarm\",\"client_exe\":\"C:\\\\evil\\\\core.exe\"}\n")
	servePipeConn(&in, &out, st)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var a, b pipeStatus
	if err := json.Unmarshal([]byte(lines[0]), &a); err != nil || a.Error != "private_endpoint" {
		t.Fatalf("private %+v %v", a, err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &b); err != nil || b.Error != "client_exe_denied" {
		t.Fatalf("unlock %+v %v", b, err)
	}
}
