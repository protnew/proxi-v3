package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	vpn "github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/nostr"
)

// handleVPNSignaling — POST publish / GET pending
// INF-010: wires VPNSignaling + Nostr relay (was initialized but unreachable).
func (s *Server) handleVPNSignaling(w http.ResponseWriter, r *http.Request) {
	if vpnSignaling == nil {
		vpnSignaling = vpn.NewVPNSignaling()
	}
	switch r.Method {
	case http.MethodGet:
		invites := vpnSignaling.GetPendingInvites()
		if invites == nil {
			invites = []vpn.VPNEvent{}
		}
		writeJSON(w, 200, map[string]interface{}{
			"pending": invites,
			"count":   len(invites),
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
		if req.From == "" || req.To == "" || req.Type == "" {
			writeError(w, 400, "VALIDATION_ERROR", "type, from, to required")
			return
		}
		var ve vpn.VPNEvent
		switch req.Type {
		case "vpn-invite":
			ve = vpnSignaling.CreateVPNInvite(req.From, req.To, req.WTAddr, req.WTCertHash)
		case "vpn-request":
			ve = vpnSignaling.CreateVPNRequest(req.From, req.To)
		case "vpn-accept":
			ve = vpnSignaling.CreateVPNAccept(req.From, req.To, req.WTAddr, req.WTCertHash)
		case "vpn-reject":
			ve = vpnSignaling.CreateVPNReject(req.From, req.To)
		case "vpn-cancel":
			ve = vpn.VPNEvent{Type: "vpn-cancel", From: req.From, To: req.To, Timestamp: time.Now().Unix()}
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
			PubKey:    req.From,
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
