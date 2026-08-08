package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/unkillable-messenger/vpn"
)

func TestTurnConfigNotConfigured(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest("GET", "/api/vpn/turn/config", nil)
	w := httptest.NewRecorder()
	srv.handleTurnConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "not_configured" {
		t.Errorf("expected not_configured, got %v", resp["status"])
	}
}

func TestAmneziaConfigGet(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest("GET", "/api/vpn/amnezia", nil)
	w := httptest.NewRecorder()
	srv.handleAmneziaConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["phase"] != "desktop_only" {
		t.Errorf("expected desktop_only, got %v", resp["phase"])
	}
}

func TestLibp2pConfigGet(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest("GET", "/api/vpn/libp2p", nil)
	w := httptest.NewRecorder()
	srv.handleLibp2pConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["phase"] != "desktop_only" {
		t.Errorf("expected desktop_only, got %v", resp["phase"])
	}
	if resp["table"] == nil {
		t.Error("expected table field")
	}
}


func TestAmneziaConfBuild(t *testing.T) {
	srv := &Server{}
	body := `{"privateKey":"AAA","peerPublicKey":"BBB","endpoint":"1.2.3.4:51820"}`
	req := httptest.NewRequest("POST", "/api/vpn/amnezia/conf", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAmneziaConfBuild(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	conf, _ := resp["conf"].(string)
	if conf == "" || !strings.Contains(conf, "Jc =") {
		t.Fatalf("bad conf: %v", resp)
	}
}

func TestPushConfig(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest("GET", "/api/push/config", nil)
	w := httptest.NewRecorder()
	srv.handlePushConfig(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestAmneziaTunnelStartDriverMissing(t *testing.T) {
	// Without kernel driver: conf must still be written, state driver_missing
	srv := &Server{}
	// unique peer key
	body := `{"peerPublicKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","endpoint":"203.0.113.10:51820"}`
	req := httptest.NewRequest("POST", "/api/vpn/amnezia/tunnel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAmneziaTunnel(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var st map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	state, _ := st["state"].(string)
	if state != "driver_missing" && state != "up" && state != "conf_ready" && state != "error" {
		t.Fatalf("unexpected state %v", st)
	}
	if state != "up" && state != "conf_ready" && state != "driver_missing" {
		t.Fatalf("tunnel not usable: %v", st)
	}
	conf, _ := st["confPath"].(string)
	if conf == "" {
		t.Fatalf("confPath empty: %v", st)
	}
	if _, err := os.Stat(conf); err != nil {
		t.Fatalf("conf not on disk: %v", err)
	}
	// GET status
	req2 := httptest.NewRequest("GET", "/api/vpn/amnezia/tunnel", nil)
	w2 := httptest.NewRecorder()
	srv.handleAmneziaTunnel(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("get %d", w2.Code)
	}
}

func TestVAPIDKeysPersisted(t *testing.T) {
	cfg := vpn.GetPushConfig()
	if !cfg.Enabled || cfg.VapidPublic == "" {
		// force
		if _, err := vpn.EnsureDevVAPID(); err != nil {
			t.Fatal(err)
		}
		cfg = vpn.GetPushConfig()
	}
	if cfg.VapidPublic == "" {
		t.Fatal("no vapid public")
	}
	// public key must be uncompressed P-256 => 65 bytes rawurl
	raw, err := base64.RawURLEncoding.DecodeString(cfg.VapidPublic)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 65 || raw[0] != 0x04 {
		t.Fatalf("not uncompressed P-256 pub, len=%d first=%x", len(raw), raw[:1])
	}
	jwt, err := vpn.SignVAPIDTest("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt parts %d", len(parts))
	}
}

