package vpn

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestHandleRPC_ConnectPassesInvite(t *testing.T) {
	pipe := &scriptPipe{}
	pipe.out.WriteString("{\"state\":\"connecting\",\"engaged\":true}\n")
	old := dialHelperPipe
	dialHelperPipe = func() (io.ReadWriteCloser, error) { return pipe, nil }
	t.Cleanup(func() { dialHelperPipe = old })

	raw := (&Manager{}).HandleRPC([]byte(`{"method":"connect_to_exit_node","params":{"endpoint":"203.0.113.9:443","token":"tok","npub":"np","sig":"sg","exp":17,"onion":"abc.onion","wtCertHash":"hash"}}`))
	if strings.Contains(string(raw), "method not found") || strings.Contains(string(raw), "helper_unavailable") {
		t.Fatalf("rpc %s", raw)
	}
	var resp struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil || resp.Result["state"] != "connecting" {
		t.Fatalf("%s", raw)
	}
	var cmd map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(pipe.in.String())), &cmd); err != nil {
		t.Fatal(err)
	}
	if cmd["token"] != "tok" || cmd["npub"] != "np" || cmd["sig"] != "sg" || cmd["exp"] != float64(17) || cmd["onion"] != "abc.onion" || cmd["wtCertHash"] != "hash" {
		t.Fatal("rpc dropped invite fields")
	}
}
