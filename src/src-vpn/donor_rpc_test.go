package vpn

import (
	"encoding/json"
	"strings"
	"testing"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{config: Config{DataDir: t.TempDir()}}
}

func TestDonorRPC_CreateInviteSeconds(t *testing.T) {
	m := testManager(t)
	raw, _ := json.Marshal(map[string]string{"to": "aa"})
	out, err := donorRPC(m, "create_invite", raw, NewExitAuth(""))
	if err != nil {
		t.Fatal(err)
	}
	body := out.(map[string]interface{})
	exp := body["exp"].(int64)
	ts := body["ts"].(int64)
	if exp <= ts || exp > ts+24*3600 {
		t.Fatalf("exp not unix seconds: ts=%d exp=%d", ts, exp)
	}
	if len(body["token"].(string)) != 32 {
		t.Fatal("token not 16 bytes hex")
	}
}

func TestDonorRPC_ConnectNeedsEndpoint(t *testing.T) {
	m := testManager(t)
	raw, _ := json.Marshal(map[string]string{"token": "t", "npub": "n", "sig": "s"})
	_, err := donorRPC(m, "connect_invite", raw, nil)
	if err == nil || !strings.Contains(err.Error(), "no_exit_peers") {
		t.Fatalf("got %v", err)
	}
}

func TestDonorRPC_StartEgressOnionOnly(t *testing.T) {
	m := testManager(t)
	out, err := donorRPC(m, "start_egress_listener", []byte(`{}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.StopEgress()
	body := out.(map[string]string)
	if body["state"] != "egress-on" {
		t.Fatalf("state=%v", body["state"])
	}
}

func TestDonorRPC_CreateInviteIssuesToken(t *testing.T) {
	m := testManager(t)
	auth := NewExitAuth("")
	raw, _ := json.Marshal(map[string]string{"to": "bb"})
	out, err := donorRPC(m, "create_invite", raw, auth)
	if err != nil {
		t.Fatal(err)
	}
	token := out.(map[string]interface{})["token"].(string)
	if auth.Issued[token].Npub != "bb" {
		t.Fatal("token not issued")
	}
}

func TestDonorRPC_CreateInviteWithoutAuth(t *testing.T) {
	m := testManager(t)
	raw, _ := json.Marshal(map[string]string{"to": "bb"})
	_, err := donorRPC(m, "create_invite", raw, nil)
	if err == nil || !strings.Contains(err.Error(), "exit auth missing") {
		t.Fatalf("got %v", err)
	}
}
