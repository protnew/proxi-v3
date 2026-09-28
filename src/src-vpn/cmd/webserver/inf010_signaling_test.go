package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	vpn "github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/nostr"
)

func TestINF010_VPNSignalingPublishAndPending(t *testing.T) {
	// Wire globals like startup
	vpnSignaling = vpn.NewVPNSignaling()
	nostrRelay = nostr.NewRelay(1000, nil)
	nostrRelay.OnEvent = func(ev nostr.Event) {
		if ev.Kind != vpn.VPNEventKind {
			return
		}
		ve, err := vpn.DeserializeVPNEvent([]byte(ev.Content))
		if err != nil {
			return
		}
		if ve.From == "" {
			ve.From = ev.PubKey
		}
		vpnSignaling.HandleIncomingEvent(ev.ID, ve)
	}

	srv := &Server{}
	body := `{"type":"vpn-invite","from":"npub_alice","to":"npub_bob","wtAddr":"127.0.0.1:4433","wtCertHash":"abc"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vpn/signaling", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleVPNSignaling(w, req)
	if w.Code != 201 {
		t.Fatalf("publish status %d body=%s", w.Code, w.Body.String())
	}
	var pub map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &pub)
	if pub["wired"] != true {
		t.Fatalf("not wired: %v", pub)
	}

	// Allow OnEvent goroutine
	deadline := time.Now().Add(2 * time.Second)
	var n int
	for time.Now().Before(deadline) {
		pending := vpnSignaling.GetPendingInvites()
		n = len(pending)
		if n > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if n == 0 {
		t.Fatal("INF-010: pending invites empty after publish — signaling not wired")
	}

	reqG := httptest.NewRequest(http.MethodGet, "/api/vpn/signaling", nil)
	wG := httptest.NewRecorder()
	srv.handleVPNSignaling(wG, reqG)
	if wG.Code != 200 {
		t.Fatalf("GET status %d", wG.Code)
	}
	var got struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(wG.Body.Bytes(), &got)
	if got.Count < 1 {
		t.Fatalf("GET count=%d", got.Count)
	}
}
