package main

import (
	"encoding/json"
	"strings"
	"net/http"
	"net/http/httptest"
	"testing"
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
