package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/identity"
	"github.com/unkillable-messenger/vpn/ipfs"
	"github.com/unkillable-messenger/vpn/nat"
	"github.com/unkillable-messenger/vpn/nostr"
	"github.com/unkillable-messenger/vpn/tor"
	"github.com/unkillable-messenger/vpn/store"

	"golang.org/x/time/rate"
	"nhooyr.io/websocket"
)

// ========== SQLite-backed store ==========

var db *store.Store

// ========== VPN Manager ==========

var vpnMgr *vpn.Manager

var nostrRelay *nostr.Relay

var ipfsClient *ipfs.Client

var torDialer *tor.TorDialer

// ========== Rate limiter ==========

var limiter = rate.NewLimiter(100, 200) // 100 req/s, burst 200

func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// ========== CORS middleware ==========

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// ========== Security headers middleware ==========

func securityHeadersMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next(w, r)
	}
}

// ========== Cache headers for static assets ==========

func setCacheHeaders(w http.ResponseWriter, path string) {
	ext := filepath.Ext(path)
	switch ext {
	case ".js", ".css", ".png", ".jpg", ".jpeg", ".svg", ".woff", ".woff2":
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case ".html", ".json":
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	case ".wasm":
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
}

// ========== MIME type mapping ==========

var mimeTypes = map[string]string{
	".js":     "application/javascript",
	".mjs":    "application/javascript",
	".css":    "text/css; charset=utf-8",
	".json":   "application/json",
	".png":    "image/png",
	".jpg":    "image/jpeg",
	".jpeg":   "image/jpeg",
	".gif":    "image/gif",
	".webp":   "image/webp",
	".svg":    "image/svg+xml",
	".html":   "text/html; charset=utf-8",
	".wasm":   "application/wasm",
	".ico":    "image/x-icon",
	".woff":   "font/woff",
	".woff2":  "font/woff2",
	".ttf":    "font/ttf",
	".webmanifest": "application/manifest+json",
	".webm":   "video/webm",
	".mp4":    "video/mp4",
	".weba":   "audio/webm",
	".ogg":    "audio/ogg",
	".pdf":    "application/pdf",
}

// ========== Response helpers ==========

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: failed to encode JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// ========== API Handlers ==========

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(startTime).Seconds(),
		"version":   "0.1.0",
	})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	vpnStatus := vpnMgr.GetStatus()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "running",
		"version": "0.1.0",
		"vpn":     string(vpnStatus.State),
		"vpnInfo": vpnStatus,
	})
}

func handleMessagesGet(w http.ResponseWriter, r *http.Request) {
	var since int64 = 0
	if v := r.URL.Query().Get("since"); v != "" {
		fmt.Sscanf(v, "%d", &since)
	}
	npub := r.URL.Query().Get("npub")
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}

	msgs, err := db.GetMessages(limit, since, npub)
	if err != nil {
		log.Printf("ERROR: db.GetMessages: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to load messages")
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": msgs,
		"count":    len(msgs),
	})
}

func handleMessagesPost(w http.ResponseWriter, r *http.Request) {
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

	// Validation
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Message text is required")
		return
	}
	if len(req.Text) > 10000 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Message too long (max 10000 chars)")
		return
	}
	if req.From == "" {
		req.From = "anonymous"
	}
	if req.To == "" {
		req.To = "broadcast"
	}

	msg := store.Message{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		From:      req.From,
		To:        req.To,
		Text:      req.Text,
		Encrypted: false,
		Timestamp: time.Now().Unix(),
	}

	// Persist to SQLite
	if err := db.SaveMessage(msg); err != nil {
		log.Printf("WARNING: failed to save message to db: %v", err)
	}

	writeJSON(w, http.StatusCreated, msg)

	log.Printf("💬 Message from %s to %s: %s", req.From, req.To, truncate(req.Text, 50))
}

func handleVpnRPC(w http.ResponseWriter, r *http.Request) {
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func containsPathTraversal(path string) bool {
	return strings.Contains(path, "..") || strings.Contains(path, "\\")
}

// ========== Server startup ==========

var startTime time.Time
var distDir string

func main() {
	startTime = time.Now()

	distDir = os.Getenv("DIST_DIR")
	if distDir == "" {
		distDir = "/app/dist"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Check dist dir exists
	if info, err := os.Stat(distDir); err != nil || !info.IsDir() {
		log.Printf("⚠️  WARNING: dist dir %s does not exist, static files will not be served", distDir)
	} else {
		log.Printf("📁 Serving static files from %s", distDir)
	}

	// Initialize SQLite store
	var dbErr error
	db, dbErr = store.NewStore(getDataDir() + "/messenger.db")
	if dbErr != nil {
		log.Fatalf("❌ Failed to initialize SQLite store: %v", dbErr)
	}
	log.Println("💾 SQLite store initialized")

	// Initialize Nostr Relay
	nostrRelay = nostr.NewRelay(50000)
	log.Println("📡 Nostr NIP-01 relay initialized")

	// Initialize IPFS client
	ipfsClient = ipfs.NewClient("", "")
	if ipfsClient.IsAvailable() {
		log.Println("📦 IPFS daemon connected")
	} else {
		log.Println("📦 IPFS daemon not found (file upload will return error)")
	}

	// Initialize Tor dialer
	torDialer = tor.NewTorDialer("")
	if torDialer.IsTorRunning() {
		log.Println("🧅 Tor SOCKS5 proxy connected")
	} else {
		log.Println("🧅 Tor not found (.onion connections disabled)")
	}

	// Initialize VPN Manager
	var vpnErr error
	vpnMgr, vpnErr = vpn.NewManager(getDataDir() + "/vpn")
	if vpnErr != nil {
		log.Fatalf("❌ Failed to initialize VPN Manager: %v", vpnErr)
	}
	log.Println("🔒 VPN Manager initialized")

	// Initialize identity — try DB first, fall back to file
	npub, _, _, idErr := db.LoadIdentity()
	if idErr != nil {
		// No identity in DB yet; try loading from legacy file
		idPath := getDataDir() + "/identity.json"
		privKey, fileErr := identity.LoadIdentity(idPath)
		if fileErr == nil {
			// Migrate file-based identity to DB
			npub = identity.PubKeyToNpub(privKey.PubKey())
			nsec := identity.PrivKeyToNsec(privKey)
			_ = db.SaveIdentity(npub, nsec, "")
			log.Printf("🔑 Identity migrated from file to DB: %s", npub)
		}
	} else {
		log.Printf("🔑 Identity loaded from DB: %s", npub)
	}

	// Initialize chat hub
	initHub()

	// Auto-connect VPN peers on startup
	go autoConnectPeers()

	// Start scheduled messages sender (every 60 seconds)
	go scheduledMessagesLoop()

	// Start dead man's switch checker (every hour)
	go deadMansSwitchLoop()

	fs := http.FileServer(http.Dir(distDir))

	// API routes with middleware chain
	apiChain := func(h http.HandlerFunc) http.HandlerFunc {
		return securityHeadersMiddleware(corsMiddleware(rateLimitMiddleware(h)))
	}

	http.HandleFunc("/api/health", apiChain(handleHealth))
	http.HandleFunc("/api/status", apiChain(handleStatus))
	http.HandleFunc("/api/messages", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handleMessagesGet(w, r)
		case "POST":
			handleMessagesPost(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
		}
	}))
	http.HandleFunc("/api/channels", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handleChannelsGet(w, r)
		case "POST":
			handleChannelsPost(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
		}
	}))
	http.HandleFunc("/api/vpn/rpc", apiChain(handleVpnRPC))
	http.HandleFunc("/api/peers", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handlePeersGet(w, r)
		case "POST":
			handlePeersAdd(w, r)
		case "DELETE":
			handlePeersRemove(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET, POST or DELETE")
		}
	}))
	// WebSocket + Identity
	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/api/identity", apiChain(handleIdentityGet))
	http.HandleFunc("/api/online", apiChain(handleOnlineUsers))

	// File upload/download
	http.HandleFunc("/api/files/upload", apiChain(handleFileUpload))
	http.HandleFunc("/api/files/", handleFileGet) // no rate limit for downloads
	// Reactions
	http.HandleFunc("/api/reactions", apiChain(handleReactions))
	// Read receipts
	http.HandleFunc("/api/read-receipts", apiChain(handleReadReceipts))
	// Profiles
	http.HandleFunc("/api/profiles", apiChain(handleProfiles))
	// Search
	http.HandleFunc("/api/search", apiChain(handleSearch))
	http.HandleFunc("/api/messages/edit", apiChain(handleEditMessage))
	http.HandleFunc("/api/messages/delete", apiChain(handleDeleteMessage))
	http.HandleFunc("/api/messages/schedule", apiChain(handleScheduleMessage))
	http.HandleFunc("/api/switch/setup", apiChain(handleSwitchSetup))
	http.HandleFunc("/api/switch/check-in", apiChain(handleSwitchCheckIn))
	http.HandleFunc("/api/push/subscribe", apiChain(handlePushSubscribe))
	http.HandleFunc("/api/groups/create", apiChain(handleGroupCreate))
	http.HandleFunc("/api/groups/members", apiChain(handleGroupMembers))
	http.HandleFunc("/api/groups/kick", apiChain(handleGroupKick))
	http.HandleFunc("/api/groups/promote", apiChain(handleGroupPromote))
	http.HandleFunc("/api/vpn/split-tunnel", apiChain(handleSplitTunnel))
	http.HandleFunc("/api/vpn/dns", apiChain(handleDNSProxy))
	http.HandleFunc("/api/nostr/stats", apiChain(handleNostrStats))
	http.HandleFunc("/nostr", handleNostrWS) // NIP-01 WebSocket endpoint
	http.HandleFunc("/api/ipfs/upload", apiChain(handleIPFSUpload))
	http.HandleFunc("/api/ipfs/status", apiChain(handleIPFSStatus))
	http.HandleFunc("/api/nat/discover", apiChain(handleNATDiscover))
	http.HandleFunc("/api/tor/status", apiChain(handleTorStatus))
	http.HandleFunc("/api/channels/subscribe", apiChain(handleChannelSubscribe))

	// Static files + SPA fallback
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Security headers for all responses
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")

		// Path traversal protection
		cleanPath := filepath.Clean(r.URL.Path)
		if containsPathTraversal(r.URL.Path) || cleanPath != r.URL.Path {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Set MIME type from map
		if ct, ok := mimeTypes[filepath.Ext(r.URL.Path)]; ok {
			w.Header().Set("Content-Type", ct)
		}

		// Set cache headers
		setCacheHeaders(w, r.URL.Path)

		// SPA fallback: serve index.html for non-file routes
		fullPath := filepath.Join(distDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(fullPath); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})

	log.Printf("🔥 Unkillable Messenger v0.1.0")
	log.Printf("   Web:  http://0.0.0.0:%s", port)
	log.Printf("   API:  http://0.0.0.0:%s/api/status", port)
	log.Printf("   Chat: http://0.0.0.0:%s/api/messages", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// autoConnectPeers loads saved VPN peers from DB and attempts to reconnect.
// Runs in background goroutine on startup.
func autoConnectPeers() {
	// Wait a moment for server to be ready
	time.Sleep(2 * time.Second)

	peers, err := db.GetPeers()
	if err != nil {
		log.Printf("🔌 Auto-connect: failed to load peers: %v", err)
		return
	}

	connected := 0
	for _, p := range peers {
		pubKey, _ := p["public_key"].(string)
		endpoint, _ := p["endpoint"].(string)
		name, _ := p["name"].(string)

		if pubKey == "" || endpoint == "" {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := vpnMgr.ConnectToExitNode(ctx, pubKey, endpoint); err != nil {
			log.Printf("🔌 Auto-connect: failed %s (%s): %v", name, endpoint, err)
		} else {
			connected++
			log.Printf("🔌 Auto-connected: %s (%s)", name, endpoint)
		}
		cancel()
	}

	if connected > 0 {
		log.Printf("🔌 Auto-connected %d/%d VPN peers", connected, len(peers))
	} else if len(peers) > 0 {
		log.Printf("🔌 Auto-connect: 0/%d peers connected (will retry on demand)", len(peers))
	}
}

// scheduledMessagesLoop checks every 60 seconds for pending scheduled messages.
func scheduledMessagesLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if db == nil || hub == nil {
			continue
		}
		msgs, err := db.GetPendingScheduled()
		if err != nil {
			log.Printf("📅 Scheduled: error: %v", err)
			continue
		}
		for _, sm := range msgs {
			// Send via WS
			chatMsg := &chat.Message{
				Type: "chat",
				From: sm.Sender,
				To:   sm.Recipient,
				Text: sm.Text,
				Ts:   time.Now().Unix(),
			}
			encoded, _ := chatMsg.Encode()
			if sm.Recipient == "broadcast" || sm.Recipient == "" {
				hub.Broadcast(encoded, "")
			} else {
				hub.SendTo(sm.Recipient, encoded)
			}
			db.MarkScheduledSent(sm.ID)
			log.Printf("📅 Scheduled sent: %s → %s", sm.ID, sm.Recipient)
		}
	}
}

// deadMansSwitchLoop checks every hour for expired switches.
func deadMansSwitchLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		if db == nil || hub == nil {
			continue
		}
		switches, err := db.GetExpiredSwitches()
		if err != nil {
			log.Printf("💀 Switch: error: %v", err)
			continue
		}
		for _, dms := range switches {
			chatMsg := &chat.Message{
				Type: "chat",
				From: "💀 dead-mans-switch",
				To:   dms.Recipient,
				Text: dms.MessageText,
				Ts:   time.Now().Unix(),
			}
			encoded, _ := chatMsg.Encode()
			hub.Broadcast(encoded, "")
			db.MarkSwitchTriggered(dms.ID)
			log.Printf("💀 Switch triggered: %s (user %s, %d days inactive)", dms.ID, dms.UserNpub[:12], dms.IntervalDays)
		}
	}
}

// handleScheduleMessage — POST /api/messages/schedule
func handleScheduleMessage(w http.ResponseWriter, r *http.Request) {
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
	if err := db.SaveScheduledMessage(sm); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	sendAtTime := time.Unix(req.SendAt, 0).Format("02.01.2006 15:04")
	writeJSON(w, 200, map[string]interface{}{
		"status":   "scheduled",
		"id":       sm.ID,
		"sendAt":   req.SendAt,
		"sendAtTime": sendAtTime,
	})
	log.Printf("📅 Scheduled: %s → %s at %s", sm.ID, req.Recipient, sendAtTime)
}

// handleSwitchSetup — POST /api/switch/setup
func handleSwitchSetup(w http.ResponseWriter, r *http.Request) {
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
	if err := db.SaveDeadMansSwitch(dms); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"status":       "created",
		"id":           dms.ID,
		"intervalDays": req.IntervalDays,
	})
	log.Printf("💀 Switch created: %s (user %s, %d days)", dms.ID, req.UserNpub[:12], req.IntervalDays)
}

// handleSwitchCheckIn — POST /api/switch/check-in
func handleSwitchCheckIn(w http.ResponseWriter, r *http.Request) {
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
	if req.UserNpub == "" {
		writeError(w, 400, "BAD_REQUEST", "userNpub required")
		return
	}
	if err := db.CheckInDeadMansSwitch(req.UserNpub); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "checked_in"})
}

// handlePushSubscribe — POST /api/push/subscribe
func handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 4*1024))
	defer r.Body.Close()

	log.Printf("🔔 Push subscription received: %s", truncate(string(body), 100))
	writeJSON(w, 200, map[string]interface{}{"status": "subscribed"})
}

// handleGroupCreate — POST /api/groups/create
func handleGroupCreate(w http.ResponseWriter, r *http.Request) {
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
	if req.Name == "" || req.CreatorNpub == "" {
		writeError(w, 400, "BAD_REQUEST", "name and creatorNpub required")
		return
	}

	groupID := fmt.Sprintf("grp-%d", time.Now().UnixNano())

	// Add creator as admin
	if err := db.SaveGroupMember(store.GroupMember{
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
		db.SaveGroupMember(store.GroupMember{
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
	db.SaveChannel(ch)

	writeJSON(w, 200, map[string]interface{}{
		"status":  "created",
		"groupId": groupID,
		"name":    req.Name,
		"members": len(req.Members) + 1,
	})
	log.Printf("👥 Group created: %s (%s) with %d members", req.Name, groupID, len(req.Members)+1)
}

// handleGroupMembers — GET /api/groups/members?groupId=...
func handleGroupMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	groupID := r.URL.Query().Get("groupId")
	if groupID == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId required")
		return
	}
	members, err := db.GetGroupMembers(groupID)
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
func handleGroupKick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST or DELETE")
		return
	}
	var req struct {
		GroupID  string `json:"groupId"`
		AdminNpub string `json:"adminNpub"`
		TargetNpub string `json:"targetNpub"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if req.GroupID == "" || req.AdminNpub == "" || req.TargetNpub == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId, adminNpub and targetNpub required")
		return
	}

	isAdmin, err := db.IsGroupAdmin(req.GroupID, req.AdminNpub)
	if err != nil || !isAdmin {
		writeError(w, 403, "FORBIDDEN", "Only admin can kick members")
		return
	}

	if err := db.RemoveGroupMember(req.GroupID, req.TargetNpub); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{"status": "kicked", "groupId": req.GroupID, "target": req.TargetNpub})
}

// handleGroupPromote — POST /api/groups/promote (admin only)
func handleGroupPromote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		GroupID     string `json:"groupId"`
		AdminNpub   string `json:"adminNpub"`
		TargetNpub  string `json:"targetNpub"`
		NewRole     string `json:"newRole"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
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

	isAdmin, err := db.IsGroupAdmin(req.GroupID, req.AdminNpub)
	if err != nil || !isAdmin {
		writeError(w, 403, "FORBIDDEN", "Only admin can promote members")
		return
	}

	if err := db.UpdateGroupMemberRole(req.GroupID, req.TargetNpub, req.NewRole); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{"status": "promoted", "newRole": req.NewRole})
}

// handleSplitTunnel — POST /api/vpn/split-tunnel
func handleSplitTunnel(w http.ResponseWriter, r *http.Request) {
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
func handleDNSProxy(w http.ResponseWriter, r *http.Request) {
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
type nhooyrWSConn struct {
	c *websocket.Conn
}

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
func handleNostrWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("[nostr] upgrade error: %v", err)
		return
	}
	nostrRelay.HandleClient(&nhooyrWSConn{c: conn})
}

// handleNostrStats returns relay statistics.
func handleNostrStats(w http.ResponseWriter, r *http.Request) {
	if nostrRelay == nil {
		writeError(w, 503, "NOT_READY", "Nostr relay not initialized")
		return
	}
	writeJSON(w, 200, nostrRelay.GetStats())
}

// ==================== IPFS Handlers ====================

// handleIPFSUpload handles file upload to IPFS.
func handleIPFSUpload(w http.ResponseWriter, r *http.Request) {
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
func handleIPFSStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"available": ipfsClient.IsAvailable(),
	})
}

// handleNATDiscover discovers public IP and NAT type via STUN.
func handleNATDiscover(w http.ResponseWriter, r *http.Request) {
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
func handleTorStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"available": torDialer.IsTorRunning(),
		"proxy":     "127.0.0.1:9050",
	})
}
