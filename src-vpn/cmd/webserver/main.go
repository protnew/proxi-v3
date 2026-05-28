package main

import (
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
	"github.com/unkillable-messenger/vpn/identity"
	"github.com/unkillable-messenger/vpn/store"

	"golang.org/x/time/rate"
)

// ========== SQLite-backed store ==========

var db *store.Store

// ========== VPN Manager ==========

var vpnMgr *vpn.Manager

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
