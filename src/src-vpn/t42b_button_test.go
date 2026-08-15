package vpn

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestT42BConfig_SystemRouteRequired(t *testing.T) {
	cfg := DefaultT42BConfig()
	if cfg.DefaultRoute != "0.0.0.0/0" {
		t.Fatalf("DefaultRoute=%q want 0.0.0.0/0", cfg.DefaultRoute)
	}
	if !cfg.KillSwitch {
		t.Fatal("kill switch must be on by default")
	}
	if cfg.Session == "" {
		t.Fatal("session name required")
	}
}

func TestT42BButton_StartWithoutUserChannel_AutoLocalExit(t *testing.T) {
	btn := NewT42BButton(DefaultT42BConfig())
	if err := btn.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer btn.Stop()

	st := btn.Status()
	if st.State != "connected" {
		t.Fatalf("state=%s want connected", st.State)
	}
	if st.SocksAddr == "" {
		t.Fatal("auto local SOCKS5 address empty")
	}
	if !st.AutoExit {
		t.Fatal("expected AutoExit=true when user did not supply a peer")
	}
	if !st.ChannelUp {
		t.Fatal("auto-exit channel must be up after Start")
	}
}

func TestT42BButton_HTTPThroughButton(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("t42b-ok"))
	}))
	defer backend.Close()

	btn := NewT42BButton(DefaultT42BConfig())
	if err := btn.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer btn.Stop()

	conn, err := btn.Dial(backend.Listener.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("GET / HTTP/1.0\r\nHost: t\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "t42b-ok") {
		t.Fatalf("body=%q", body)
	}
}

func TestT42BButton_KillSwitchDropsWhenChannelDown(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("should-not-see"))
	}))
	defer backend.Close()

	btn := NewT42BButton(DefaultT42BConfig())
	if err := btn.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer btn.Stop()

	btn.SetChannelUp(false) // peer DataChannel dropped
	if btn.Status().ChannelUp {
		t.Fatal("channel still up after SetChannelUp(false)")
	}

	conn, err := btn.Dial(backend.Listener.Addr().String())
	if err == nil {
		conn.Close()
		t.Fatal("Dial must fail when channel is down — otherwise traffic leaks")
	}
	if !strings.Contains(err.Error(), "kill switch") {
		t.Fatalf("error should mention kill switch, got %v", err)
	}

	// backend itself is still reachable directly — leak would be Dial succeeding, not this
	c2, err := net.DialTimeout("tcp", backend.Listener.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("direct dial to backend failed (test infra): %v", err)
	}
	c2.Close()
}

func TestT42BButton_StopDisconnects(t *testing.T) {
	btn := NewT42BButton(DefaultT42BConfig())
	if err := btn.Start(); err != nil {
		t.Fatal(err)
	}
	if err := btn.Stop(); err != nil {
		t.Fatal(err)
	}
	st := btn.Status()
	if st.State != "disconnected" {
		t.Fatalf("state=%s", st.State)
	}
	if _, err := btn.Dial("127.0.0.1:9"); err == nil {
		t.Fatal("Dial after Stop must fail")
	}
}

func TestT42BButton_Restart(t *testing.T) {
	btn := NewT42BButton(DefaultT42BConfig())
	if err := btn.Start(); err != nil {
		t.Fatal(err)
	}
	if err := btn.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := btn.Start(); err != nil {
		t.Fatalf("restart Start: %v", err)
	}
	defer btn.Stop()
	if btn.Status().State != "connected" {
		t.Fatal("not connected after restart")
	}
}
