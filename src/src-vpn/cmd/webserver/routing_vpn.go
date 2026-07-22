// File: routing_vpn.go
// P2-1 RESCUE 20260720: extracted from routing.go (God Object split).

package main

import (

	"encoding/json"

	"io"

	"net/http"

	"time"
	"github.com/go-playground/validator/v10"
	"github.com/unkillable-messenger/vpn"

)

func (s *Server) handleVpnRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read body")
		return
	}
	defer r.Body.Close()

	resp := vpnMgr.HandleRPC(body)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// ========== Utils ==========

var startTime time.Time

var distDir string


var validate = validator.New()


func (s *Server) handleSplitTunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		Mode    string   `json:"mode"`
		Targets []string `json:"targets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.Mode != "all" && req.Mode != "split" && req.Mode != "exclude" {
		writeError(w, 400, "BAD_REQUEST", "mode must be all, split or exclude")
		return
	}

	cfg := vpn.SplitTunnelConfig{
		Mode:    req.Mode,
		Targets: req.Targets,
	}
	if err := vpnMgr.SetSplitTunnel(cfg); err != nil {
		writeError(w, 500, "SPLIT_ERROR", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"status":  "configured",
		"mode":    req.Mode,
		"targets": len(req.Targets),
	})
}

// handleDNSProxy — POST /api/vpn/dns


func (s *Server) handleDNSProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		Enabled  bool   `json:"enabled"`
		Listen   string `json:"listen"`
		Upstream string `json:"upstream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}

	cfg := vpn.DNSConfig{
		Enabled:  req.Enabled,
		Listen:   req.Listen,
		Upstream: req.Upstream,
	}
	if err := vpnMgr.StartDNSProxy(cfg); err != nil {
		writeError(w, 500, "DNS_ERROR", err.Error())
		return
	}

	status := "stopped"
	if cfg.Enabled {
		status = "running"
	}
	writeJSON(w, 200, map[string]interface{}{
		"status":   status,
		"listen":   cfg.Listen,
		"upstream": cfg.Upstream,
	})
}

// ==================== Nostr NIP-01 Handlers ====================

// nhooyrWSConn adapts nhooyr.io/websocket.Conn to nostr.WebSocketConn.


