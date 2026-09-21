package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	vpn "github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/nostr"
)

// handleVPNSignaling — POST publish / GET pending
// INF-010: wires VPNSignaling + Nostr relay (was initialized but unreachable).
// P1 (2026-09-21): identity is taken from JWT claims only — body `from` is
// ignored when auth is enabled; GET returns only the caller's own invites.
func (s *Server) handleVPNSignaling(w http.ResponseWriter, r *http.Request) {
	if vpnSignaling == nil {
		vpnSignaling = vpn.NewVPNSignaling()
	}
	// P1: resolve caller identity from JWT (npub → x-only hex).
	callerHex := ""
	if s.authService != nil {
		npub, _ := r.Context().Value("npub").(string)
		if npub == "" {
			writeError(w, 401, "UNAUTHORIZED", "identity required")
			return
		}
		h, err := npubToXOnlyHex(npub)
		if err != nil {
			writeError(w, 401, "UNAUTHORIZED", "invalid identity in token")
			return
		}
		callerHex = h
	}
	switch r.Method {
	case http.MethodGet:
		invites := vpnSignaling.GetPendingInvites()
		own := make([]vpn.VPNEvent, 0, len(invites))
		for _, inv := range invites {
			if callerHex == "" || strings.EqualFold(inv.From, callerHex) || strings.EqualFold(inv.To, callerHex) {
				own = append(own, inv)
			}
		}
		writeJSON(w, 200, map[string]interface{}{
			"pending": own,
			"count":   len(own),
		})
	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
		if err != nil {
			writeError(w, 400, "READ_ERROR", "bad body")
			return
		}
		defer r.Body.Close()
		var req struct {
			Type       string `json:"type"` // vpn-invite|vpn-request|vpn-accept|vpn-reject|vpn-cancel
			From       string `json:"from"`
			To         string `json:"to"`
			WTAddr     string `json:"wtAddr"`
			WTCertHash string `json:"wtCertHash"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, 400, "PARSE_ERROR", "invalid JSON")
			return
		}
		if req.To == "" || req.Type == "" {
			writeError(w, 400, "VALIDATION_ERROR", "type, to required")
			return
		}
		// P1: `from` comes from JWT claims, never from the body.
		from := req.From
		if callerHex != "" {
			from = callerHex
		}
		if from == "" {
			writeError(w, 401, "UNAUTHORIZED", "identity required")
			return
		}
		var ve vpn.VPNEvent
		switch req.Type {
		case "vpn-invite":
			ve = vpnSignaling.CreateVPNInvite(from, req.To, req.WTAddr, req.WTCertHash)
		case "vpn-request":
			ve = vpnSignaling.CreateVPNRequest(from, req.To)
		case "vpn-accept":
			ve = vpnSignaling.CreateVPNAccept(from, req.To, req.WTAddr, req.WTCertHash)
		case "vpn-reject":
			ve = vpnSignaling.CreateVPNReject(from, req.To)
		case "vpn-cancel":
			ve = vpn.VPNEvent{Type: "vpn-cancel", From: from, To: req.To, Timestamp: time.Now().Unix()}
		default:
			writeError(w, 400, "VALIDATION_ERROR", "unknown type")
			return
		}
		content, err := vpn.SerializeVPNEvent(ve)
		if err != nil {
			writeError(w, 500, "SERIALIZE_ERROR", err.Error())
			return
		}
		ev := nostr.Event{
			PubKey:    from,
			CreatedAt: ve.Timestamp,
			Kind:      vpn.VPNEventKind,
			Tags:      [][]string{{"p", req.To}},
			Content:   string(content),
			Sig:       "local", // INF-010 local inject; production signs with nsec
		}
		// Inject into relay → OnEvent → vpnSignaling.HandleIncomingEvent
		if nostrRelay != nil {
			nostrRelay.InjectLocalEvent(ev)
		} else {
			// Fallback: direct handle without relay
			id := nostr.ComputeEventID(&ev)
			vpnSignaling.HandleIncomingEvent(id, ve)
		}
		log.Printf("INF-010: published %s %s→%s", ve.Type, truncate(ve.From, 12), truncate(ve.To, 12))
		writeJSON(w, 201, map[string]interface{}{
			"status":  "published",
			"event":   ve,
			"kind":    vpn.VPNEventKind,
			"wired":   true,
		})
	default:
		writeError(w, 405, "METHOD_NOT_ALLOWED", "GET or POST")
	}
}
