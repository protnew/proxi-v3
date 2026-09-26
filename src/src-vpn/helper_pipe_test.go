package vpn

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type scriptPipe struct {
	in  bytes.Buffer
	out bytes.Buffer
}

func (s *scriptPipe) Write(p []byte) (int, error) { return s.in.Write(p) }
func (s *scriptPipe) Read(p []byte) (int, error)  { return s.out.Read(p) }

func TestQueryHelper_ReadsStatus(t *testing.T) {
	pipe := &scriptPipe{}
	pipe.out.WriteString("{\"state\":\"off\",\"engaged\":false}\n")
	st, err := QueryHelper(pipe)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "off" || st.Engaged {
		t.Fatalf("%+v", st)
	}
}

func TestRPCConnect_WritesPipeAndReadsStatus(t *testing.T) {
	pipe := &scriptPipe{}
	pipe.out.WriteString("{\"state\":\"connecting\",\"engaged\":true}\n")
	st, err := ConnectHelper(pipe, "203.0.113.9:443", false)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "connecting" || !st.Engaged {
		t.Fatalf("%+v", st)
	}
	var cmd map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(pipe.in.String())), &cmd); err != nil {
		t.Fatal(err)
	}
	if cmd["verb"] != "connect" || cmd["endpoint"] != "203.0.113.9:443" {
		t.Fatalf("wire %v", cmd)
	}
}
