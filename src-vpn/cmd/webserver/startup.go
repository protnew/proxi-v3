package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"encoding/hex"
	"os/signal"
	"syscall"

	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/auth"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/federation"
	"github.com/unkillable-messenger/vpn/identity"
	"github.com/unkillable-messenger/vpn/ipfs"
	"github.com/unkillable-messenger/vpn/mesh"
	"github.com/unkillable-messenger/vpn/nostr"
	"github.com/unkillable-messenger/vpn/storage"
	"github.com/unkillable-messenger/vpn/store"
	"github.com/unkillable-messenger/vpn/tor"
	"go.uber.org/zap"
)

func main() {
	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it, relying on system env vars")
	}

	// Initialize Sentry (no-op if SENTRY_DSN is empty)
	sentryDSN := os.Getenv("SENTRY_DSN")
	if sentryDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:           sentryDSN,
			EnableTracing: false,
			Release:       "unkillable-messenger@0.1.0",
			Environment:   os.Getenv("SENTRY_ENVIRONMENT"),
		})
		if err != nil {
			log.Printf("⚠️  Sentry init failed: %v", err)
		} else {
			log.Println("🔍 Sentry error tracking enabled")
		}
		// Ensure Sentry flushes events on exit
		defer sentry.Flush(2 * time.Second)
	} else {
		log.Println("🔍 Sentry disabled (SENTRY_DSN not set)")
	}

	// Initialize Zap structured logging
	logger, _ := zap.NewProduction()
	defer logger.Sync() // flushes buffer, if any
	zap.ReplaceGlobals(logger)
	zap.RedirectStdLog(logger)
	log.Println("🚀 Unkillable Messenger initializing...")

	// Wrap main logic in a deferred recover so panics are reported to Sentry
	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
			log.Fatalf("💥 Panic recovered: %v", r)
		}
	}()

	if err := run(); err != nil {
		sentry.CaptureException(err)
		sentry.Flush(2 * time.Second)
		log.Fatalf("💥 Fatal: %v", err)
	}
}

func run() error {
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

	// Start embedded IPFS node manager
	ipfsNode, err := ipfs.StartNode(context.Background())
	if err != nil {
		log.Printf("⚠️  IPFS node start warning: %v", err)
	} else {
		defer ipfsNode.Stop()
	}

	// Initialize SQLite store
	dbPath := os.Getenv("DB_PATH")
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}
	if dbPath == "" {
		os.MkdirAll(dataDir, 0755)
		dbPath = path.Join(dataDir, "messenger.db")
	}
	db, err = store.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	log.Println("✅ Database initialized")

	// Start Dead Man's Switch worker
	go startDeadMansSwitchWorker(context.Background(), db)

	var spErr error
	storageProvider, spErr = storage.NewLocalStore(filepath.Join(dataDir, "uploads"))
	if spErr != nil {
		return fmt.Errorf("failed to init storage: %w", spErr)
	}

	// Initialize Chat Hub
	initHub()

	// Initialize P2P Mesh Network
	maxHops := 6
	if envHops := os.Getenv("MESH_MAX_HOPS"); envHops != "" {
		fmt.Sscanf(envHops, "%d", &maxHops)
	}
	meshNet = mesh.NewMeshNet(maxHops)
	meshNet.StartPruner(context.Background(), 5*time.Minute, 30*time.Minute)
	log.Printf("🕸️  P2P Mesh Network initialized (maxHops=%d, prune every 5m, stale after 30m)", maxHops)

	// Initialize Nostr Relay
	nostrRelay = nostr.NewRelay(50000, db)
	log.Println("📡 Nostr NIP-01 relay initialized")

	// Initialize Federation Relay
	fedRelay = federation.NewFederatedRelay(nostrRelay)
	// Load persisted federation peers from DB
	fedPeers, fedPeerErr := db.GetFederationPeers()
	if fedPeerErr != nil {
		log.Printf("⚠️  Failed to load federation peers: %v", fedPeerErr)
	} else {
		for _, fp := range fedPeers {
			if err := fedRelay.AddPeer(fp.URL); err == nil {
				log.Printf("🌐 Federation peer restored: %s (%s)", fp.URL, fp.Status)
			}
		}
		if len(fedPeers) > 0 {
			log.Printf("🌐 Federation relay initialized with %d peer(s)", len(fedPeers))
		} else {
			log.Println("🌐 Federation relay initialized (no peers)")
		}
	}

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
		return fmt.Errorf("failed to initialize VPN Manager: %w", vpnErr)
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
			if err := db.SaveIdentity(npub, nsec, ""); err != nil {
				log.Printf("Error saving identity: %v", err)
			}
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
	publicApiChain := func(h http.HandlerFunc) http.HandlerFunc {
		return securityHeadersMiddleware(corsMiddleware(rateLimitMiddleware(h)))
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		secret = hex.EncodeToString(b)
		log.Printf("WARNING: JWT_SECRET not set, generated random secret")
	}
	globalAuthService = auth.NewAuthService(secret)
	apiChain := func(h http.HandlerFunc) http.HandlerFunc {
		return publicApiChain(authMiddleware(globalAuthService, h))
	}

	http.HandleFunc("/api/health", publicApiChain(handleHealth))
	http.HandleFunc("/api/status", publicApiChain(handleStatus))
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
	http.HandleFunc("/api/contacts", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handleContactsGet(w, r)
		case "POST":
			handleContactsSave(w, r)
		case "DELETE":
			handleContactsRemove(w, r)
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
	
	// Media endpoints (v12 content_manifests)
	http.HandleFunc("/api/media/upload", apiChain(handleMediaUpload))
	http.HandleFunc("/api/media/", handleMediaGet)
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
	http.HandleFunc("/api/groups/list", apiChain(handleGroupList))
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

	// Stream endpoints (Sprint 3 — Task 1)
	http.HandleFunc("/api/stream/create", apiChain(handleStreamCreate))
	http.HandleFunc("/api/stream/list", apiChain(handleStreamList))
	http.HandleFunc("/api/stream/end", apiChain(handleStreamEnd))
	http.HandleFunc("/api/stream/subscribe", apiChain(handleStreamSubscribe))

	// Bot endpoints (Sprint 3 — Task 2)
	http.HandleFunc("/api/bots/register", apiChain(handleBotRegister))
	http.HandleFunc("/api/bots/list", apiChain(handleBotList))

	// Sticker endpoints (Sprint 3 — Task 2)
	http.HandleFunc("/api/stickers/packs", apiChain(handleStickerPacks))
	http.HandleFunc("/api/stickers/pack/", apiChain(handleStickerPackGet))

	// Mesh network endpoints
	http.HandleFunc("/api/mesh/peers", apiChain(handleMeshPeers))
	http.HandleFunc("/api/mesh/stats", apiChain(handleMeshStats))
	http.HandleFunc("/api/mesh/add", apiChain(handleMeshAdd))

	// Federation endpoints (Sprint 6 — S6.1)
	http.HandleFunc("/api/federation/peer", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			handleFederationPeerAdd(w, r)
		case "DELETE":
			handleFederationPeerRemove(w, r)
		case "GET":
			handleFederationPeerList(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET, POST or DELETE")
		}
	}))
	http.HandleFunc("/api/federation/sync", apiChain(handleFederationSync))

	// Auth endpoints (D1 — JWT authentication)
	http.HandleFunc("/api/auth/signup", publicApiChain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
			return
		}
		var body struct {
			Npub     string `json:"npub"`
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if body.Npub == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "npub required")
			return
		}
		userID := hex.EncodeToString([]byte(body.Npub))[:16]
		username := body.Username
		if username == "" {
			username = "user_" + userID[:8]
		}
		db.DB().Exec("INSERT OR IGNORE INTO users (id, npub, username, created_at) VALUES (?, ?, ?, ?)",
			userID, body.Npub, username, time.Now().Unix())
		accessToken, refreshToken, err := globalAuthService.GenerateTokenPair(userID, body.Npub)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", err.Error())
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": accessToken, "refresh_token": refreshToken, "user_id": userID})
	}))
	http.HandleFunc("/api/auth/login", publicApiChain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
			return
		}
		var body struct {
			Npub string `json:"npub"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		var userID, npub string
		err := db.DB().QueryRow("SELECT id, npub FROM users WHERE npub = ?", body.Npub).Scan(&userID, &npub)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "NOT_FOUND", "user not found")
			return
		}
		accessToken, refreshToken, _ := globalAuthService.GenerateTokenPair(userID, npub)
		json.NewEncoder(w).Encode(map[string]string{"access_token": accessToken, "refresh_token": refreshToken, "user_id": userID})
	}))
	http.HandleFunc("/api/auth/refresh", publicApiChain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
			return
		}
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		newAccess, newRefresh, err := globalAuthService.RefreshToken(body.RefreshToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": newAccess, "refresh_token": newRefresh})
	}))

	initExtraRoutes(db, apiChain)

	// Static files + SPA fallback
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Security headers for all responses
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")

		// Path traversal protection
		cleanPath := path.Clean(r.URL.Path)
		if strings.Contains(r.URL.Path, "..") || cleanPath != r.URL.Path {
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

	srv := &http.Server{Addr: ":" + port}

	// Graceful shutdown on SIGINT/SIGTERM
	idleConnsClosed := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("📴 Received %s, shutting down gracefully...", sig)
		if hub != nil {
			hub.RawBroadcastJSON(map[string]string{"type": "system", "text": "server shutting down"}, "")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
		close(idleConnsClosed)
	}()

	// HTTPS if TLS certs configured
	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")
	if tlsCert != "" && tlsKey != "" {
		log.Printf("   TLS:  https (certs: %s)", tlsCert)
		if err := srv.ListenAndServeTLS(tlsCert, tlsKey); err != http.ErrServerClosed {
			return fmt.Errorf("HTTPS server error: %w", err)
		}
	} else {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			return fmt.Errorf("HTTP server error: %w", err)
		}
	}
	
	<-idleConnsClosed
	log.Println("✅ Server stopped")
	return nil
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
			log.Printf("💀 Switch triggered: %s (user %s, %d days inactive)", dms.ID, truncate(dms.UserNpub, 12), dms.IntervalDays)
		}
	}
}

// handleScheduleMessage — POST /api/messages/schedule
