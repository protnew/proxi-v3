// AUDIT-C5: This file is 1282 lines — exceeds 500 line limit.
// TODO: Split into:
//   routing_core.go     — router setup, middleware, helpers
//   routing_chat.go     — message/dm/typing handlers  
//   routing_groups.go   — group management handlers
//   routing_vpn.go      — VPN/WireGuard/split-tunnel/DNS handlers
//   routing_mesh.go     — mesh/stream/IPFS/NAT/Tor handlers

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/bot"

	"github.com/unkillable-messenger/vpn/federation"
	"github.com/unkillable-messenger/vpn/mesh"
	"github.com/unkillable-messenger/vpn/middleware"

	"github.com/unkillable-messenger/vpn/nostr"
	"github.com/unkillable-messenger/vpn/storage"
	"github.com/unkillable-messenger/vpn/store"
	"github.com/unkillable-messenger/vpn/stream"
	"github.com/unkillable-messenger/vpn/tor"

	"nhooyr.io/websocket"
)


var storageProvider storage.StorageProvider

// ========== VPN Manager ==========

var vpnMgr *vpn.Manager

var nostrRelay *nostr.Relay

var fedRelay *federation.FederatedRelay

var torDialer *tor.TorDialer

var meshNet *mesh.MeshNet

var streamMgr = stream.NewManager()

var botMgr = bot.NewManager()

var stickerMgr = bot.NewStickerManager()

// ========== Per-user rate limiter ==========

var perUserLimiter = middleware.NewRateLimiter(100, 200) // 100 req/s, burst 200

// ========== CORS Policy ==========

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// P0-3 RESCUE 20260720: whitelist-based CORS (no origin reflection).
		applyCORSHeaders(w, r, true)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

var mimeTypes = map[string]string{
	".js":          "application/javascript",
	".mjs":         "application/javascript",
	".css":         "text/css; charset=utf-8",
	".json":        "application/json",
	".png":         "image/png",
	".jpg":         "image/jpeg",
	".jpeg":        "image/jpeg",
	".gif":         "image/gif",
	".webp":        "image/webp",
	".svg":         "image/svg+xml",
	".html":        "text/html; charset=utf-8",
	".wasm":        "application/wasm",
	".ico":         "image/x-icon",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
	".ttf":         "font/ttf",
	".webmanifest": "application/manifest+json",
	".webm":        "video/webm",
	".mp4":         "video/mp4",
	".weba":        "audio/webm",
	".ogg":         "audio/ogg",
	".pdf":         "application/pdf",
}

// ========== Response helpers ==========

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(startTime).Seconds(),
		"version":   "0.1.0",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	vpnStatus := vpnMgr.GetStatus()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "running",
		"version": "0.1.0",
		"vpn":     string(vpnStatus.State),
		"vpnInfo": vpnStatus,
	})
}

func (s *Server) handleSwitchSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		UserNpub     string `json:"userNpub"`
		MessageText  string `json:"messageText"`
		Recipient    string `json:"recipient"`
		IntervalDays int    `json:"intervalDays"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.UserNpub == "" || req.MessageText == "" {
		writeError(w, 400, "BAD_REQUEST", "userNpub and messageText required")
		return
	}
	if req.IntervalDays < 1 {
		req.IntervalDays = 7
	}
	if req.Recipient == "" {
		req.Recipient = "broadcast"
	}

	dms := store.DeadMansSwitch{
		ID:           fmt.Sprintf("dms-%d-%s", time.Now().UnixNano(), randomHex(4)),
		UserNpub:     req.UserNpub,
		MessageText:  req.MessageText,
		Recipient:    req.Recipient,
		IntervalDays: req.IntervalDays,
		LastCheckIn:  time.Now().Unix(),
		CreatedAt:    time.Now().Unix(),
	}
	if err := s.db.SaveDeadMansSwitch(dms); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"status":       "created",
		"id":           dms.ID,
		"intervalDays": req.IntervalDays,
	})
	log.Printf("💀 Switch created: %s (user %s, %d days)", dms.ID, truncate(req.UserNpub, 12), req.IntervalDays)
}

// handleSwitchCheckIn — POST /api/switch/check-in

func (s *Server) handleSwitchCheckIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		UserNpub string `json:"userNpub"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.UserNpub == "" {
		writeError(w, 400, "BAD_REQUEST", "userNpub required")
		return
	}
	if err := s.db.CheckInDeadMansSwitch(req.UserNpub); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "checked_in"})
}

// handlePushSubscribe — POST /api/push/subscribe

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 4*1024))
	defer r.Body.Close()

	log.Printf("🔔 Push subscription received: %s", truncate(string(body), 100))
	writeJSON(w, 200, map[string]interface{}{"status": "subscribed"})
}

// handleGroupList — GET /api/groups/list

func (g *nhooyrWSConn) ReadJSON(v interface{}) error {
	_, r, err := g.c.Reader(context.Background())
	if err != nil {
		return err
	}
	return json.NewDecoder(r).Decode(v)
}

func (g *nhooyrWSConn) WriteJSON(v interface{}) error {
	w, err := g.c.Writer(context.Background(), websocket.MessageText)
	if err != nil {
		return err
	}
	err = json.NewEncoder(w).Encode(v)
	w.Close()
	return err
}

func (g *nhooyrWSConn) Close() error {
	return g.c.Close(websocket.StatusNormalClosure, "")
}

// handleNostrWS handles WebSocket connections for the Nostr relay.

func (s *Server) handleNostrStats(w http.ResponseWriter, r *http.Request) {
	if nostrRelay == nil {
		writeError(w, 503, "NOT_READY", "Nostr relay not initialized")
		return
	}
	writeJSON(w, 200, nostrRelay.GetStats())
}

// ==================== IPFS Handlers ====================

// handleIPFSUpload handles file upload to IPFS.

func (s *Server) handleBotRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		Name      string `json:"name"`
		OwnerNpub string `json:"ownerNpub"`
		Command   string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, 400, "BAD_REQUEST", "name required")
		return
	}
	if req.OwnerNpub == "" {
		req.OwnerNpub = "anonymous"
	}
	reg := botMgr.RegisterBot(req.Name, req.OwnerNpub, req.Command, &bot.EchoBot{})
	writeJSON(w, 201, reg)
	log.Printf("🤖 Bot registered: %s (%s) by %s", req.Name, req.Command, req.OwnerNpub)
}

// handleBotList — GET /api/bots/list

func (s *Server) handleBotList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	bots := botMgr.ListBots()
	if bots == nil {
		bots = []bot.BotRegistration{}
	}
	writeJSON(w, 200, map[string]interface{}{
		"bots":  bots,
		"count": len(bots),
	})
}

// ==================== Sticker Handlers (Sprint 3 — Task 2) ====================

// handleStickerPacks — GET /api/stickers/packs

func (s *Server) handleStickerPacks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	packs := stickerMgr.ListPacks()
	if packs == nil {
		packs = []bot.StickerPack{}
	}
	writeJSON(w, 200, map[string]interface{}{
		"packs": packs,
		"count": len(packs),
	})
}

// handleStickerPackGet — GET /api/stickers/pack/:id

func (s *Server) handleStickerPackGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	// Extract ID from path: /api/stickers/pack/:id
	packID := strings.TrimPrefix(r.URL.Path, "/api/stickers/pack/")
	if packID == "" {
		writeError(w, 400, "BAD_REQUEST", "pack ID required")
		return
	}
	pack, err := stickerMgr.GetPack(packID)
	if err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, 200, pack)
}

// ==================== Federation Handlers (Sprint 6 — S6.1) ====================

// handleFederationPeerAdd — POST /api/federation/peer

