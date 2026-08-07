package main

import (
	"encoding/json"
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
