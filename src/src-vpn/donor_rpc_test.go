package vpn

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDonorRPC_CreateInviteSeconds(t *testing.T) {
	raw, _ := json.Marshal(map[string]string{"to": "aa"})
	out, err := donorRPC("create_invite", raw)
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
	raw, _ := json.Marshal(map[string]string{"token": "t", "npub": "n"})
	_, err := donorRPC("connect_invite", raw)
	if err == nil || !strings.Contains(err.Error(), "no_exit_peers") {
		t.Fatalf("got %v", err)
	}
}

func TestDonorRPC_StartNeedsBind(t *testing.T) {
	_, err := donorRPC("start_egress_listener", []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "public_bind") {
		t.Fatalf("got %v", err)
	}
}
