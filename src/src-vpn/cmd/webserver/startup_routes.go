// LEGACY: /api/vpn/wt/* is LAN-only WebTransport (Table 01 primary = WebRTC). Do not expand.
// File: startup_routes.go
// P2-2 RESCUE 20260721: extracted from startup.go run().
// All HTTP route registrations in one place.

package main

import (
	"strconv"
	vpnroot "github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/auth"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// registerRoutes sets up all HTTP endpoints on the default mux.
// Called from run() in startup.go.
func (srv *Server) registerRoutes(authSvc *auth.AuthService, distDir, port string) {
	// Middleware chains
	// QA-004 (2026-08-11): apiChain теперь требует JWT когда authSvc != nil
	// Это закрывает rebinding атаку — все API endpoints требуют Bearer token
	apiChain := func(h http.HandlerFunc) http.HandlerFunc {
		return securityHeadersMiddleware(corsMiddleware(rateLimitMiddleware(h)))
	}
	// publicApiChain: NEVER requires JWT — health/status/signup/login/refresh
	// BUGFIX AUTH-004 (2026-07-27): previously when authSvc!=nil, publicApiChain
	// incorrectly wrapped authMiddleware → POST /api/auth/signup always 401.
	publicApiChain := func(h http.HandlerFunc) http.HandlerFunc {
		return securityHeadersMiddleware(corsMiddleware(h))
	}
	// protectedApiChain: JWT required (use for identity/messages as routes migrate)
	protectedApiChain := apiChain
	if authSvc != nil {
		protectedApiChain = func(h http.HandlerFunc) http.HandlerFunc {
			return securityHeadersMiddleware(corsMiddleware(rateLimitMiddleware(authMiddleware(authSvc, h))))
		}
		// QA-004: Harden apiChain to also require JWT — closes rebinding vulnerability
		apiChain = protectedApiChain
	}
	// AUTH-009: protectedApiChain used for messages/identity below

	http.HandleFunc("/api/health", publicApiChain(srv.handleHealth))
	http.HandleFunc("/api/network/lan", publicApiChain(handleLANInfo))
	http.HandleFunc("/api/status", protectedApiChain(srv.handleStatus))
	// AUTH-009: messages require JWT when auth is enabled
	http.HandleFunc("/api/messages", protectedApiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			srv.handleMessagesGet(w, r)
		case "POST":
			srv.handleMessagesPost(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
		}
	}))
	http.HandleFunc("/api/channels", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			srv.handleChannelsGet(w, r)
		case "POST":
			srv.handleChannelsPost(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
		}
	}))
		// INF-010: VPN Nostr signaling (kind:30090)
	http.HandleFunc("/api/vpn/signaling", apiChain(srv.handleVPNSignaling))

http.HandleFunc("/api/vpn/rpc", apiChain(srv.handleVpnRPC))
	http.HandleFunc("/api/vpn/wt/stats", apiChain(srv.handleWTStats))
	http.HandleFunc("/api/vpn/wt/start", apiChain(srv.handleWTStart))
	http.HandleFunc("/api/vpn/wt/stop", apiChain(srv.handleWTStop))
	http.HandleFunc("/api/vpn/turn/config", apiChain(srv.handleTurnConfig))
	http.HandleFunc("/api/vpn/amnezia", apiChain(srv.handleAmneziaConfig))
	http.HandleFunc("/api/vpn/amnezia/conf", apiChain(srv.handleAmneziaConfBuild))
	http.HandleFunc("/api/vpn/amnezia/tunnel", apiChain(srv.handleAmneziaTunnel))
	http.HandleFunc("/api/vpn/amnezia/import", apiChain(srv.handleAmneziaImport))
	http.HandleFunc("/api/vpn/libp2p", apiChain(srv.handleLibp2pConfig))
	http.HandleFunc("/api/contacts", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			srv.handleContactsGet(w, r)
		case "POST":
			srv.handleContactsSave(w, r)
		case "DELETE":
			srv.handleContactsRemove(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET, POST or DELETE")
		}
	}))
	// WebSocket + Identity
	http.HandleFunc("/ws", srv.handleWS)
	http.HandleFunc("/api/identity", protectedApiChain(srv.handleIdentityGet))
	http.HandleFunc("/api/online", apiChain(srv.handleOnlineUsers))

	// File upload/download
	http.HandleFunc("/api/files/upload", apiChain(srv.handleFileUpload))
	// S1 leftover (2026-09-01): list + DELETE need JWT. GET /api/files/{id} stays
	// public so chat attachments work as bare URLs (img src / window.open, no header).
	http.HandleFunc("/api/files", protectedApiChain(srv.handleFileGet))
	http.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/files/")
		if path == "" || r.Method == http.MethodDelete {
			protectedApiChain(srv.handleFileGet)(w, r)
			return
		}
		publicApiChain(srv.handleFileGet)(w, r)
	})
	
	// Media endpoints (v12 content_manifests)
	http.HandleFunc("/api/media/upload", apiChain(srv.handleMediaUpload))
	http.HandleFunc("/api/media/", srv.handleMediaGet)
	// Reactions
	http.HandleFunc("/api/reactions", apiChain(srv.handleReactions))
	// Read receipts
	http.HandleFunc("/api/read-receipts", apiChain(srv.handleReadReceipts))
	// Profiles
	http.HandleFunc("/api/profiles", apiChain(srv.handleProfiles))
	// Search
	http.HandleFunc("/api/search", apiChain(srv.handleSearch))
	http.HandleFunc("/api/messages/edit", apiChain(srv.handleEditMessage))
	http.HandleFunc("/api/messages/delete", apiChain(srv.handleDeleteMessage))
	http.HandleFunc("/api/messages/schedule", apiChain(srv.handleScheduleMessage))
	http.HandleFunc("/api/switch/setup", apiChain(srv.handleSwitchSetup))
	http.HandleFunc("/api/switch/check-in", apiChain(srv.handleSwitchCheckIn))
	http.HandleFunc("/api/push/config", apiChain(srv.handlePushConfig))
	http.HandleFunc("/api/push/subscribe", apiChain(srv.handlePushSubscribe))
	http.HandleFunc("/api/push/send", apiChain(srv.handlePushSend))
	// CRYP-011: X3DH+DR session establish
	http.HandleFunc("/api/keys/session", apiChain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			srv.handleSessionEstablish(w, r)
			return
		}
		srv.handleSessionStatus(w, r)
	}))

	// N7: PreKey bundle distribution (desktop X3DH)
	http.HandleFunc("/api/keys/prekey", apiChain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			srv.handlePreKeyPublish(w, r)
		} else {
			srv.handlePreKeyGet(w, r)
		}
	}))
	http.HandleFunc("/api/keys/prekey/", apiChain(srv.handlePreKeyGet))
	http.HandleFunc("/api/groups/create", apiChain(srv.handleGroupCreate))
	http.HandleFunc("/api/groups/list", apiChain(srv.handleGroupList))
	http.HandleFunc("/api/groups/members", apiChain(srv.handleGroupMembers))
	http.HandleFunc("/api/groups/kick", apiChain(srv.handleGroupKick))
	http.HandleFunc("/api/groups/promote", apiChain(srv.handleGroupPromote))
	http.HandleFunc("/api/vpn/split-tunnel", apiChain(srv.handleSplitTunnel))
	http.HandleFunc("/api/vpn/dns", apiChain(srv.handleDNSProxy))
	http.HandleFunc("/api/nostr/stats", apiChain(srv.handleNostrStats))
	http.HandleFunc("/nostr", srv.handleNostrWS) // NIP-01 WebSocket endpoint
	http.HandleFunc("/api/ipfs/upload", apiChain(srv.handleIPFSUpload))
	http.HandleFunc("/api/ipfs/status", apiChain(srv.handleIPFSStatus))
	http.HandleFunc("/api/nat/discover", apiChain(srv.handleNATDiscover))
	http.HandleFunc("/api/tor/status", apiChain(srv.handleTorStatus))
	http.HandleFunc("/api/channels/subscribe", apiChain(srv.handleChannelSubscribe))

	// Stream endpoints (Sprint 3 — Task 1)
	http.HandleFunc("/api/stream/create", apiChain(srv.handleStreamCreate))
	http.HandleFunc("/api/stream/list", apiChain(srv.handleStreamList))
	http.HandleFunc("/api/stream/end", apiChain(srv.handleStreamEnd))
	http.HandleFunc("/api/stream/subscribe", apiChain(srv.handleStreamSubscribe))

	// Bot endpoints (Sprint 3 — Task 2)
	http.HandleFunc("/api/bots/register", apiChain(srv.handleBotRegister))
	http.HandleFunc("/api/bots/list", apiChain(srv.handleBotList))

	// Sticker endpoints (Sprint 3 — Task 2)
	http.HandleFunc("/api/stickers/packs", apiChain(srv.handleStickerPacks))
	http.HandleFunc("/api/stickers/pack/", apiChain(srv.handleStickerPackGet))

	// Mesh network endpoints
	http.HandleFunc("/api/mesh/peers", apiChain(srv.handleMeshPeers))
	http.HandleFunc("/api/mesh/stats", apiChain(srv.handleMeshStats))
	http.HandleFunc("/api/mesh/add", apiChain(srv.handleMeshAdd))

	// Federation endpoints (Sprint 6 — S6.1)
	http.HandleFunc("/api/federation/peer", apiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			srv.handleFederationPeerAdd(w, r)
		case "DELETE":
			srv.handleFederationPeerRemove(w, r)
		case "GET":
			srv.handleFederationPeerList(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET, POST or DELETE")
		}
	}))
	http.HandleFunc("/api/federation/sync", apiChain(srv.handleFederationSync))

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
		// SEC-002: shared validator (vpnroot.ValidateSignupInput)
		if err := vpnroot.ValidateSignupInput(body.Username, body.Npub); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		// userID: prefer hex prefix of npub (64-char hex pubkey); fallback hash-encode
		userID := body.Npub
		if len(userID) >= 16 {
			userID = userID[:16]
		} else {
			userID = hex.EncodeToString([]byte(body.Npub))
			if len(userID) > 16 {
				userID = userID[:16]
			}
		}
		username := body.Username
		if username == "" {
			username = "user_" + userID[:8]
		}
		w.Header().Set("Content-Type", "application/json")
		srv.db.DB().Exec("INSERT OR IGNORE INTO users (id, npub, username, created_at) VALUES (?, ?, ?, ?)",
			userID, body.Npub, username, time.Now().Unix())
		accessToken, refreshToken, err := srv.authService.GenerateTokenPair(userID, body.Npub)
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
		err := srv.db.DB().QueryRow("SELECT id, npub FROM users WHERE npub = ?", body.Npub).Scan(&userID, &npub)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "NOT_FOUND", "user not found")
			return
		}
		accessToken, refreshToken, _ := srv.authService.GenerateTokenPair(userID, npub)
		w.Header().Set("Content-Type", "application/json")
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
		newAccess, newRefresh, err := srv.authService.RefreshToken(body.RefreshToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"access_token": newAccess, "refresh_token": newRefresh})
	}))

	initExtraRoutes(srv.db, apiChain)

	// Static files + SPA fallback (Windows-safe: os.ReadFile avoids http.FileServer Cyrillic issues)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; connect-src 'self' ws: wss:; img-src 'self' data: blob:; manifest-src 'self'; worker-src 'self';")

		if strings.Contains(r.URL.Path, "..") {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Strip leading / so filepath.Join doesn't treat as absolute
		urlPath := strings.TrimPrefix(r.URL.Path, "/")
		urlPath = strings.TrimPrefix(urlPath, "\\")
		if urlPath == "" {
			urlPath = "index.html"
		}
		cleanPath := filepath.Clean(urlPath)
		fullPath := filepath.Join(distDir, cleanPath)

		data, err := os.ReadFile(fullPath)
		if err != nil {
			data, err = os.ReadFile(filepath.Join(distDir, "index.html"))
			if err != nil {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		} else {
			ext := strings.ToLower(filepath.Ext(fullPath))
			switch ext {
			case ".js":
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			case ".css":
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			case ".json", ".webmanifest":
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
			case ".svg":
				w.Header().Set("Content-Type", "image/svg+xml")
			case ".png":
				w.Header().Set("Content-Type", "image/png")
			case ".ico":
				w.Header().Set("Content-Type", "image/x-icon")
			case ".woff2":
				w.Header().Set("Content-Type", "font/woff2")
			case ".html":
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			default:
				w.Header().Set("Content-Type", "application/octet-stream")
			}
			if strings.HasPrefix(cleanPath, string(os.PathSeparator)+"assets"+string(os.PathSeparator)) {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	})
}
