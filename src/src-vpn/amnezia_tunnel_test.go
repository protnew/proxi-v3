package vpn

import (
	"os"
	"strings"
	"testing"
)

func TestStartAmneziaTunnelUserspace(t *testing.T) {
	os.Setenv("DATA_DIR", t.TempDir())
	st, err := StartAmneziaTunnel(TunnelStartRequest{
		PeerPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Endpoint:      "203.0.113.10:51820",
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.ConfPath == "" {
		t.Fatal("no conf path")
	}
	b, err := os.ReadFile(st.ConfPath)
	if err != nil {
		t.Fatal(err)
	}
	conf := string(b)
	if !strings.Contains(conf, "Jc =") || !strings.Contains(conf, "PrivateKey") {
		t.Fatalf("bad conf: %s", conf)
	}
	if st.State != "up" && st.State != "conf_ready" {
		t.Fatalf("state=%s note=%s err=%s", st.State, st.Note, st.LastError)
	}
	// stop
	st2 := StopAmneziaTunnel()
	if st2.State != "stopped" {
		t.Fatalf("stop state %s", st2.State)
	}
}

func TestVAPIDProductionKeyFormat(t *testing.T) {
	cfg := GetPushConfig()
	if cfg.VapidPublic == "" {
		if _, err := EnsureDevVAPID(); err != nil {
			t.Fatal(err)
		}
		cfg = GetPushConfig()
	}
	if !cfg.Enabled {
		t.Fatal("not enabled")
	}
	jwt, err := SignVAPIDTest("https://push.example")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(jwt, ".") != 2 {
		t.Fatalf("jwt %s", jwt)
	}
}
