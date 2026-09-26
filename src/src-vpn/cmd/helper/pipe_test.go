package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const testCore = `C:\Program Files\Proxi\proxi04-core.exe`

func TestPipe_SecondConnSeesConnect(t *testing.T) {
	st := newHelperState()
	st.corePath = testCore
	var in1, out1 bytes.Buffer
	in1.WriteString("{\"verb\":\"connect\",\"endpoint\":\"203.0.113.9:443\"}\n")
	servePipeConn(&in1, &out1, st, testCore)

	var in2, out2 bytes.Buffer
	in2.WriteString("{\"verb\":\"status\"}\n")
	servePipeConn(&in2, &out2, st, `C:\evil\other.exe`)

	var got pipeStatus
	if err := json.Unmarshal([]byte(strings.TrimSpace(out2.String())), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != "connecting" || !got.Engaged || got.Endpoint != "203.0.113.9:443" {
		t.Fatalf("second conn lost state: %+v", got)
	}
}

func TestPipe_ForeignJSONDoesNotAuthorize(t *testing.T) {
	st := newHelperState()
	st.corePath = testCore
	var in, out bytes.Buffer
	in.WriteString("{\"verb\":\"connect\",\"endpoint\":\"203.0.113.9:443\",\"client_exe\":\"C:\\\\Program Files\\\\Proxi\\\\proxi04-core.exe\",\"token\":\"sek\"}\n")
	in.WriteString("{\"verb\":\"disarm\",\"client_exe\":\"C:\\\\Program Files\\\\Proxi\\\\proxi04-core.exe\"}\n")
	servePipeConn(&in, &out, st, `C:\evil\other.exe`)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines %d", len(lines))
	}
	for i, line := range lines {
		var got pipeStatus
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatal(err)
		}
		if got.Error != "client_exe_denied" || got.Engaged {
			t.Fatalf("line %d %+v", i, got)
		}
	}
	if st.token != "" {
		t.Fatal("token stored from foreign")
	}
}

func TestPipe_CoreConnectKeepsInvite(t *testing.T) {
	st := newHelperState()
	st.corePath = testCore
	var in, out bytes.Buffer
	in.WriteString("{\"verb\":\"connect\",\"endpoint\":\"203.0.113.9:443\",\"onion\":\"abc.onion\",\"wtCertHash\":\"h\",\"token\":\"tok\",\"npub\":\"np\",\"sig\":\"sg\",\"exp\":17}\n")
	servePipeConn(&in, &out, st, `\\?\`+testCore)
	if st.token != "tok" || st.npub != "np" || st.sig != "sg" || st.exp != 17 || st.onion != "abc.onion" || st.wtCertHash != "h" {
		t.Fatal("dropped invite fields")
	}
	var got pipeStatus
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != "connecting" || !got.Engaged {
		t.Fatalf("%+v", got)
	}
}

func TestPipe_PrivateFromCore(t *testing.T) {
	st := newHelperState()
	st.corePath = testCore
	var in, out bytes.Buffer
	in.WriteString("{\"verb\":\"connect\",\"endpoint\":\"192.168.1.1:443\"}\n")
	servePipeConn(&in, &out, st, testCore)
	var got pipeStatus
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error != "private_endpoint" {
		t.Fatalf("%+v", got)
	}
}
