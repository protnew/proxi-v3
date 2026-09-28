package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHTTPBindAddrLoopback(t *testing.T) {
	if got := httpBindAddr("8090"); got != "127.0.0.1:8090" {
		t.Fatalf("httpBindAddr(8090)=%q", got)
	}
	if got := httpBindAddr(""); got != "127.0.0.1:8090" {
		t.Fatalf("httpBindAddr empty=%q", got)
	}
	if !httpBindIsLoopback("127.0.0.1:8090") {
		t.Fatal("127.0.0.1 should be loopback")
	}
	if httpBindIsLoopback("0.0.0.0:8090") {
		t.Fatal("0.0.0.0 is not loopback")
	}
}

func TestHandleLANInfoBindMatchesListen(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/network/lan", nil)
	w := httptest.NewRecorder()
	handleLANInfo(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	bind, _ := body["bind"].(string)
	if bind != "127.0.0.1:8090" {
		t.Fatalf("bind=%q want 127.0.0.1:8090", bind)
	}
	if bind == "0.0.0.0:8090" {
		t.Fatal("bind still advertises 0.0.0.0")
	}
	if body["reachable_from_lan"] != false {
		t.Fatalf("reachable_from_lan=%v want false", body["reachable_from_lan"])
	}
	urls, _ := body["phone_urls"].([]interface{})
	if len(urls) != 0 {
		t.Fatalf("phone_urls=%v want empty on loopback bind", urls)
	}
}
