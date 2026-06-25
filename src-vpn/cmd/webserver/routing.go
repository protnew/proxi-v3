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

	"github.com/go-playground/validator/v10"
	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/bot"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/crypto"
	"github.com/unkillable-messenger/vpn/federation"
	"github.com/unkillable-messenger/vpn/mesh"
	"github.com/unkillable-messenger/vpn/middleware"
	"github.com/unkillable-messenger/vpn/nat"
	"github.com/unkillable-messenger/vpn/nostr"
	"github.com/unkillable-messenger/vpn/storage"
	"github.com/unkillable-messenger/vpn/store"
	"github.com/unkillable-messenger/vpn/stream"
	"github.com/unkillable-messenger/vpn/tor"
	"go.uber.org/zap"

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
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept")
		w.Header().Set("Access-Control-Max-Age", "86400")

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

func (s *Server) handleMessagesGet(w http.ResponseWriter, r *http.Request) {
	var since int64 = 0
	if v := r.URL.Query().Get("since"); v != "" {
		fmt.Sscanf(v, "%d", &since)
	}
	npub := r.URL.Query().Get("npub")
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}

	msgs, err := s.db.GetMessages(limit, since, npub)
	if err != nil {
		log.Printf("ERROR: s.db.GetMessages: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to load messages")
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}

	// Auto-decrypt messages for the current user
	currentUserNpub, _ := r.Context().Value("npub").(string)
	if currentUserNpub != "" {
		recipientBundle, err := s.db.GetPreKeyBundle(currentUserNpub)
		if err == nil && recipientBundle != nil {
			privBundle := &crypto.PreKeyBundle{
				IdentityKey: recipientBundle.IdentityKey,
			}
			for i, m := range msgs {
				if m.Encrypted && m.To == currentUserNpub {
					plaintext, err := chat.DecryptMessageFromSender(m.Text, privBundle.IdentityKey, m.From)
					if err == nil {
						msgs[i].Text = plaintext
						msgs[i].Encrypted = false // Mark as decrypted for the UI
					} else {
						zap.S().Warnf("Failed to decrypt message %s from %s: %v", m.ID, m.From, err)
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": msgs,
		"count":    len(msgs),
	})
}

func (s *Server) handleMessagesPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST to send messages")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024)) // 64KB max
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}

	// Validation
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Message text is required")
		return
	}
	if len(req.Text) > 10000 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Message too long (max 10000 chars)")
		return
	}
	if req.To == "" {
		req.To = "broadcast"
	}

	// Force sender from context (JWT token) to prevent spoofing
	if userNpub, ok := r.Context().Value("npub").(string); ok && userNpub != "" {
		req.From = userNpub
	} else if req.From == "" {
		req.From = "anonymous"
	}

	// Enable E2E Encryption
	encryptedText := req.Text
	isEncrypted := false
	if req.To != "broadcast" {
		// Fetch sender's prekey bundle
		bundleRow, err := s.db.GetPreKeyBundle(req.From)
		if err == nil && bundleRow != nil {
			senderBundle := &crypto.PreKeyBundle{
				IdentityKey: bundleRow.IdentityKey,
			}
			enc, err := chat.EncryptMessageForRecipient(req.Text, senderBundle.IdentityKey, req.To)
			if err == nil {
				encryptedText = enc
				isEncrypted = true
			} else {
				log.Printf("WARNING: E2E encryption failed: %v", err)
			}
		}
	}

	msg := store.Message{
		ID:        fmt.Sprintf("msg-%d-%s", time.Now().UnixNano(), req.From[:min(8, len(req.From))]),
		From:      req.From,
		To:        req.To,
		Text:      encryptedText,
		Encrypted: isEncrypted,
		Timestamp: time.Now().Unix(),
	}

	// Persist to SQLite
	if err := s.db.SaveMessage(msg); err != nil {
		log.Printf("WARNING: failed to save message to db: %v", err)
	}

	writeJSON(w, http.StatusCreated, msg)

	// Redact plaintext messages from logs (Compliance P0)
	log.Printf("💬 Message from %s to %s (%d bytes)", req.From, req.To, len(req.Text))
}

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

func (s *Server) handleScheduleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		Text      string `json:"text"`
		SendAt    int64  `json:"sendAt"`
		Recipient string `json:"recipient"`
		Sender    string `json:"sender"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.Text == "" || req.SendAt == 0 {
		writeError(w, 400, "BAD_REQUEST", "text and sendAt required")
		return
	}
	if req.SendAt <= time.Now().Unix() {
		writeError(w, 400, "BAD_REQUEST", "sendAt must be in the future")
		return
	}
	if req.Recipient == "" {
		req.Recipient = "broadcast"
	}
	if req.Sender == "" {
		req.Sender = "anonymous"
	}

	sm := store.ScheduledMessage{
		ID:        fmt.Sprintf("sched-%d-%s", time.Now().UnixNano(), randomHex(4)),
		Sender:    req.Sender,
		Recipient: req.Recipient,
		Text:      req.Text,
		SendAt:    req.SendAt,
		Status:    "pending",
		CreatedAt: time.Now().Unix(),
	}
	if err := s.db.SaveScheduledMessage(sm); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	sendAtTime := time.Unix(req.SendAt, 0).Format("02.01.2006 15:04")
	writeJSON(w, 200, map[string]interface{}{
		"status":     "scheduled",
		"id":         sm.ID,
		"sendAt":     req.SendAt,
		"sendAtTime": sendAtTime,
	})
	log.Printf("📅 Scheduled: %s → %s at %s", sm.ID, req.Recipient, sendAtTime)
}

// handleSwitchSetup — POST /api/switch/setup

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

func (s *Server) handleGroupList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	channels, err := s.db.GetChannels()
	if err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}
	groups := make([]map[string]interface{}, 0)
	for _, ch := range channels {
		groups = append(groups, map[string]interface{}{
			"id":        ch.ID,
			"name":      ch.Name,
			"creator":   ch.Creator,
			"createdAt": ch.CreatedAt,
		})
	}
	writeJSON(w, 200, map[string]interface{}{
		"groups": groups,
		"count":  len(groups),
	})
}

// handleGroupCreate — POST /api/groups/create

func (s *Server) handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		Name        string   `json:"name"`
		CreatorNpub string   `json:"creatorNpub"`
		Members     []string `json:"members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.Name == "" || req.CreatorNpub == "" {
		writeError(w, 400, "BAD_REQUEST", "name and creatorNpub required")
		return
	}

	groupID := fmt.Sprintf("grp-%d", time.Now().UnixNano())

	// Add creator as admin
	if err := s.db.SaveGroupMember(store.GroupMember{
		GroupID:  groupID,
		UserNpub: req.CreatorNpub,
		Role:     "admin",
		JoinedAt: time.Now().Unix(),
	}); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	// Add members
	for _, m := range req.Members {
		if m == req.CreatorNpub {
			continue
		}
		s.db.SaveGroupMember(store.GroupMember{
			GroupID:  groupID,
			UserNpub: m,
			Role:     "member",
			JoinedAt: time.Now().Unix(),
		})
	}

	// Also create as a channel for message routing
	ch := store.Channel{
		ID:          groupID,
		Name:        req.Name,
		Description: "Group chat",
		Creator:     req.CreatorNpub,
		Subscribers: len(req.Members) + 1,
		CreatedAt:   time.Now().Unix(),
	}
	s.db.SaveChannel(ch)

	writeJSON(w, 200, map[string]interface{}{
		"status":  "created",
		"groupId": groupID,
		"name":    req.Name,
		"members": len(req.Members) + 1,
	})
	log.Printf("👥 Group created: %s (%s) with %d members", req.Name, groupID, len(req.Members)+1)
}

// handleGroupMembers — GET /api/groups/members?groupId=...

func (s *Server) handleGroupMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	groupID := r.URL.Query().Get("groupId")
	if groupID == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId required")
		return
	}
	members, err := s.db.GetGroupMembers(groupID)
	if err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}
	if members == nil {
		members = []store.GroupMember{}
	}
	writeJSON(w, 200, map[string]interface{}{
		"groupId": groupID,
		"members": members,
		"count":   len(members),
	})
}

// handleGroupKick — DELETE /api/groups/kick (admin only)

func (s *Server) handleGroupKick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST or DELETE")
		return
	}
	var req struct {
		GroupID    string `json:"groupId"`
		AdminNpub  string `json:"adminNpub"`
		TargetNpub string `json:"targetNpub"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.GroupID == "" || req.AdminNpub == "" || req.TargetNpub == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId, adminNpub and targetNpub required")
		return
	}

	isAdmin, err := s.db.IsGroupAdmin(req.GroupID, req.AdminNpub)
	if err != nil || !isAdmin {
		writeError(w, 403, "FORBIDDEN", "Only admin can kick members")
		return
	}

	if err := s.db.RemoveGroupMember(req.GroupID, req.TargetNpub); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{"status": "kicked", "groupId": req.GroupID, "target": req.TargetNpub})
}

// handleGroupPromote — POST /api/groups/promote (admin only)

func (s *Server) handleGroupPromote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		GroupID    string `json:"groupId"`
		AdminNpub  string `json:"adminNpub"`
		TargetNpub string `json:"targetNpub"`
		NewRole    string `json:"newRole"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.GroupID == "" || req.AdminNpub == "" || req.TargetNpub == "" || req.NewRole == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId, adminNpub, targetNpub and newRole required")
		return
	}
	if req.NewRole != "admin" && req.NewRole != "moderator" && req.NewRole != "member" {
		writeError(w, 400, "BAD_REQUEST", "newRole must be admin, moderator or member")
		return
	}

	isAdmin, err := s.db.IsGroupAdmin(req.GroupID, req.AdminNpub)
	if err != nil || !isAdmin {
		writeError(w, 403, "FORBIDDEN", "Only admin can promote members")
		return
	}

	if err := s.db.UpdateGroupMemberRole(req.GroupID, req.TargetNpub, req.NewRole); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{"status": "promoted", "newRole": req.NewRole})
}

// handleSplitTunnel — POST /api/vpn/split-tunnel

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

func (s *Server) handleIPFSUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST with multipart form")
		return
	}

	if !ipfsClient.IsAvailable() {
		writeError(w, 503, "IPFS_UNAVAILABLE", "IPFS daemon not running")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "BAD_REQUEST", "No file provided")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, 500, "READ_ERROR", err.Error())
		return
	}

	result, err := ipfsClient.UploadFile(header.Filename, data)
	if err != nil {
		writeError(w, 500, "UPLOAD_ERROR", err.Error())
		return
	}

	// Pin the file
	ipfsClient.PinFile(result.CID)

	writeJSON(w, 200, map[string]interface{}{
		"cid":        result.CID,
		"gatewayUrl": result.GatewayURL,
		"size":       result.Size,
		"filename":   header.Filename,
	})
}

// handleIPFSStatus returns IPFS daemon status.

func (s *Server) handleIPFSStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"available": ipfsClient.IsAvailable(),
	})
}

// handleNATDiscover discovers public IP and NAT type via STUN.

func (s *Server) handleNATDiscover(w http.ResponseWriter, r *http.Request) {
	result, err := nat.DiscoverPublicAddr("")
	if err != nil {
		writeError(w, 500, "STUN_ERROR", err.Error())
		return
	}
	natType, _ := nat.DetectNATType("")
	result.NATType = natType
	writeJSON(w, 200, result)
}

// handleTorStatus returns Tor SOCKS5 proxy status.

func (s *Server) handleTorStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"available": torDialer.IsTorRunning(),
		"proxy":     "127.0.0.1:9050",
	})
}

// ==================== Mesh Network Handlers ====================

// handleMeshPeers — GET /api/mesh/peers

func (s *Server) handleMeshPeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	peers := meshNet.GetAllPeers()
	if peers == nil {
		peers = []mesh.PeerInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

// handleMeshStats — GET /api/mesh/stats

func (s *Server) handleMeshStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	writeJSON(w, http.StatusOK, meshNet.GetStats())
}

// handleMeshAdd — POST /api/mesh/add

func (s *Server) handleMeshAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var peer mesh.PeerInfo
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if peer.ID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "peer id is required")
		return
	}
	meshNet.AddPeer(peer)
	log.Printf("🕸️  Mesh peer added: %s (%s)", peer.ID, peer.Address)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "added",
		"id":     peer.ID,
	})
}

// ==================== Stream Handlers (Sprint 3 — Task 1) ====================

// handleStreamCreate — POST /api/stream/create

func (s *Server) handleStreamCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		ChannelName string `json:"channelName"`
		StreamerID  string `json:"streamerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.StreamerID == "" {
		req.StreamerID = "anonymous"
	}
	if req.ChannelName == "" {
		req.ChannelName = "Live Stream"
	}
	streamObj := streamMgr.CreateStream(req.ChannelName, req.StreamerID)
	writeJSON(w, 201, s)
	log.Printf("📺 Stream created: %s by %s (%s)", streamObj.ID, req.StreamerID, req.ChannelName)
}

// handleStreamList — GET /api/stream/list

func (s *Server) handleStreamList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	streams := streamMgr.ListStreams()
	if streams == nil {
		streams = []stream.Stream{}
	}
	writeJSON(w, 200, map[string]interface{}{
		"streams": streams,
		"count":   len(streams),
	})
}

// handleStreamEnd — POST /api/stream/end

func (s *Server) handleStreamEnd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		StreamID string `json:"streamId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.StreamID == "" {
		writeError(w, 400, "BAD_REQUEST", "streamId required")
		return
	}
	if err := streamMgr.EndStream(req.StreamID); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "ended", "streamId": req.StreamID})
	log.Printf("📺 Stream ended: %s", req.StreamID)
}

// handleStreamSubscribe — POST /api/stream/subscribe

func (s *Server) handleStreamSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		StreamID string `json:"streamId"`
		UserID   string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.StreamID == "" {
		writeError(w, 400, "BAD_REQUEST", "streamId required")
		return
	}
	if err := streamMgr.Subscribe(req.StreamID, req.UserID); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "subscribed", "streamId": req.StreamID})
}

// ==================== Bot Handlers (Sprint 3 — Task 2) ====================

// handleBotRegister — POST /api/bots/register

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

func (s *Server) handleFederationPeerAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "url is required")
		return
	}

	// Add to in-memory federation relay
	if err := fedRelay.AddPeer(req.URL); err != nil {
		writeError(w, http.StatusConflict, "DUPLICATE", err.Error())
		return
	}

	// Persist to DB
	peer := store.FederationPeer{
		ID:       fmt.Sprintf("fp-%d", time.Now().UnixNano()),
		URL:      req.URL,
		LastSync: 0,
		Status:   "active",
	}
	if err := s.db.SaveFederationPeer(peer); err != nil {
		log.Printf("⚠️  Failed to persist federation peer: %v", err)
	}

	log.Printf("🌐 Federation peer added: %s", req.URL)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "added",
		"url":    req.URL,
	})
}

// handleFederationPeerRemove — DELETE /api/federation/peer

func (s *Server) handleFederationPeerRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use DELETE")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "url is required")
		return
	}

	// Remove from in-memory relay
	if err := fedRelay.RemovePeer(req.URL); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	// Remove from DB
	if err := s.db.DeleteFederationPeer(req.URL); err != nil {
		log.Printf("⚠️  Failed to delete federation peer from DB: %v", err)
	}

	log.Printf("🌐 Federation peer removed: %s", req.URL)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "removed",
		"url":    req.URL,
	})
}

// handleFederationPeerList — GET /api/federation/peer

func (s *Server) handleFederationPeerList(w http.ResponseWriter, r *http.Request) {
	peers, err := s.db.GetFederationPeers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if peers == nil {
		peers = []store.FederationPeer{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

// handleFederationSync — POST /api/federation/sync

func (s *Server) handleFederationSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		Since int64 `json:"since"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default: sync from 0 (full sync)
		req.Since = 0
	}

	if err := fedRelay.SyncEvents(context.Background(), req.Since); err != nil {
		writeError(w, http.StatusInternalServerError, "SYNC_ERROR", err.Error())
		return
	}

	// Update last_sync for all peers in DB
	now := time.Now().Unix()
	peers := fedRelay.GetPeers()
	for _, p := range peers {
		if err := s.db.UpdateFederationPeerSync(p, now); err != nil {
			log.Printf("⚠️  Failed to update sync time for %s: %v", p, err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "synced",
		"peerCount": len(peers),
		"syncedAt":  now,
	})
}

func startDeadMansSwitchWorker(ctx context.Context, s *Server) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			switches, err := s.db.GetExpiredSwitches()
			if err != nil {
				zap.S().Errorf("Failed to check expired switches: %v", err)
				continue
			}

			for _, dms := range switches {
				zap.S().Infof("💀 Triggering Dead Man's Switch %s for user %s", dms.ID, dms.UserNpub)
				// Broadcast via hub if possible, or save as a system message to recipient
				msg := store.Message{
					ID:        fmt.Sprintf("dms-%d", time.Now().UnixNano()),
					From:      dms.UserNpub,
					To:        dms.Recipient,
					Text:      dms.MessageText,
					Timestamp: time.Now().Unix(),
				}
				s.db.SaveMessage(msg)

				if s.hub != nil {
					s.hub.RawBroadcastJSON(msg, "")
				}

				dms.Triggered = true
				s.db.SaveDeadMansSwitch(dms)
			}
		}
	}
}
