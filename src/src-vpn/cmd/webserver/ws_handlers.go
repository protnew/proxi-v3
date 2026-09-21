package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/unkillable-messenger/vpn/bot"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/identity"
	"github.com/unkillable-messenger/vpn/store"

	"nhooyr.io/websocket"
)


// handleWS upgrades HTTP to WebSocket and registers client.
// AUTH-009: when authService is set, a valid JWT is required.
// Routing identity is claims.Npub (full pubkey) so DM to/from match hub keys.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// P19: Origin whitelist — CSWSH через произвольный Origin закрыт.
	if origin := r.Header.Get("Origin"); origin != "" && !wsOriginAllowed(origin) {
		http.Error(w, `{"error":"FORBIDDEN","message":"origin not allowed"}`, http.StatusForbidden)
		return
	}
	userId := ""
	tokenStr := r.URL.Query().Get("token")
	if s.authService != nil {
		if tokenStr == "" {
			http.Error(w, `{"error":"UNAUTHORIZED","message":"token required"}`, http.StatusUnauthorized)
			return
		}
		claims, err := s.authService.ValidateToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"UNAUTHORIZED","message":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		// Prefer full npub for DM routing; fallback to UserID
		if claims.Npub != "" {
			userId = claims.Npub
		} else {
			userId = claims.UserID
		}
	} else {
		// Legacy open mode (tests / no auth)
		if tokenStr != "" {
			userId = tokenStr
		}
		if userId == "" {
			userId = r.URL.Query().Get("userId")
		}
		if userId == "" {
			userId = "anon-" + randomHex(4)
		}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("WS accept error: %v", err)
		return
	}

	log.Printf("🔌 WS connected: %s", truncate(userId, 16))
	chat.ServeWS(s.hub, userId, conn, r.Context())
}

// handleIdentityGet returns current user's identity (generates if needed).
// Singleton device identity: only the owner JWT user may export nsec.
func (s *Server) handleIdentityGet(w http.ResponseWriter, r *http.Request) {
	caller, _ := r.Context().Value("userID").(string)
	if caller == "" {
		caller, _ = r.Context().Value("user_id").(string)
	}

	// Try loading from DB first
	npub, _, _, err := s.db.LoadIdentity()
	if err == nil && npub != "" {
		owner := s.db.LoadIdentityOwner()
		if owner == "" && caller != "" {
			_ = s.db.SetIdentityOwner(caller)
			owner = caller
		}
		if owner != "" && caller != "" && owner != caller {
			writeError(w, 403, "FORBIDDEN", "identity owned by another user")
			return
		}
		// P3: private key material never leaves the server over the API.
		writeJSON(w, 200, map[string]interface{}{
			"npub":  npub,
			"isNew": false,
		})
		return
	}

	// No identity in DB — generate new one
	privKey, _, genErr := identity.GenerateKeyPair()
	if genErr != nil {
		writeError(w, 500, "KEYGEN_ERROR", "Failed to generate keys")
		return
	}

	mnemonic, _ := identity.GenerateMnemonic()
	nsec := identity.PrivKeyToNsec(privKey)
	npub = identity.PubKeyToNpub(privKey.PubKey())

	// Save to SQLite
	if saveErr := s.db.SaveIdentity(npub, nsec, mnemonic); saveErr != nil {
		log.Printf("WARNING: failed to save identity to db: %v", saveErr)
	}
	if caller != "" {
		if ownErr := s.db.SetIdentityOwner(caller); ownErr != nil {
			log.Printf("WARNING: failed to set identity owner: %v", ownErr)
		}
	}

	// P3: no plaintext identity.json next to the DB — SQLite is the store.

	// P3: private key material never leaves the server over the API.
	writeJSON(w, 200, map[string]interface{}{
		"npub":  npub,
		"isNew": true,
	})
}

// handleOnlineUsers returns list of connected users
func (s *Server) handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	users := s.hub.OnlineUsers()
	if users == nil {
		users = []string{}
	}
	writeJSON(w, 200, map[string]interface{}{
		"users": users,
		"count": len(users),
	})
}

// initHub creates the chat hub
func (s *Server) initHub() {
	s.hub = chat.NewChatHub()
	if s.drSessions == nil {
		s.drSessions = chat.NewDRSessionStore() // CRYP-010
	}
	if s.db != nil {
		s.drSessions.SetMarker(s.db) // P13: persist session markers, fail closed after restart
	}
	s.hub.OnMessage = func(msg *chat.Message) {
		// P5: never log message bodies on the hot path (metadata only).
		log.Printf("💬 [%s→%s] type=%s encrypted=%v len=%d", msg.From, msg.To, msg.Type, msg.ClaimedEncrypted(), len(msg.Text))

		// Process bot commands — skip ciphertext (bots need plaintext; E2E DMs are opaque).
		if msg.Type == "chat" && msg.Text != "" && !msg.ClaimedEncrypted() && !chat.LooksLikeClientCiphertext(msg.Text) {
			botMsg := bot.Message{
				ID:        msg.ID,
				From:      msg.From,
				To:        msg.To,
				Text:      msg.Text,
				Timestamp: msg.Ts,
				Type:      msg.Type,
			}
			triggered := botMgr.ProcessMessage(botMsg)
			if len(triggered) > 0 {
				log.Printf("🤖 Triggered %d bot(s) for message from %s", len(triggered), msg.From)
			}
		}

		// Save chat messages to SQLite (skip empty text — MSG-003)
		// P5: persist ciphertext as-is with Encrypted=true; refuse claimed-encrypted plaintext.
		if msg.Type == "chat" && strings.TrimSpace(msg.Text) != "" {
			if msg.ClaimedEncrypted() && !chat.LooksLikeClientCiphertext(msg.Text) {
				log.Printf("P5 refuse persist: encrypted flag without ciphertext from %s", truncate(msg.From, 16))
				return
			}
			msg.NormalizeE2EFlags()
			msgID := fmt.Sprintf("msg-%d-%s", msg.Ts, randomHex(4))
			storeMsg := store.Message{
				ID:            msgID,
				From:          msg.From,
				To:            msg.To,
				Text:          msg.Text,
				Encrypted:     msg.ClaimedEncrypted(),
				Timestamp:     msg.Ts,
				ReplyTo:       msg.ReplyTo,
				ForwardedFrom: msg.ForwardedFrom,
				TTL:           msg.TTL,
			}
			// Enrich reply with preview text
			if msg.ReplyTo != "" {
				if orig, err := s.db.GetMessageByID(msg.ReplyTo); err == nil {
					msg.ReplyToText = orig.Text
					msg.ReplyToFrom = orig.From
					if len(orig.Text) > 80 {
						msg.ReplyToText = orig.Text[:80] + "..."
					}
				}
			}
			// Assign server-generated ID back to message for WS broadcast
			msg.ID = msgID
			if err := s.db.SaveMessage(storeMsg); err != nil {
				log.Printf("ERROR: save message: %v", err)
			}
		}
	}
	log.Println("💬 Chat Hub initialized")

	// MSG-005: one-shot cleanup of empty historical bubbles
	if s.db != nil {
		if n, err := s.db.DeleteEmptyMessages(false); err == nil && n > 0 {
			log.Printf("🧹 Removed %d empty messages from DB", n)
		}
	}

	// Start self-destruct cleaner (runs every 30 seconds)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if s.db == nil {
				continue
			}
			if n, err := s.db.CleanExpiredMessages(); err == nil && n > 0 {
				log.Printf("💣 Self-destruct: cleaned %d expired messages", n)
			}
		}
	}()
}

// ========== Peer Management Handlers ==========

// isAdminIdentity — P7: WireGuard mesh changes (AddPeer/RemovePeer) are
// restricted to identities listed in PROXI_ADMIN_NPUBS (comma-separated npub
// or 64-hex). Empty list = nobody may change the mesh via API.
var (
	adminNpubsOnce sync.Once
	adminNpubs     map[string]bool
)

func isAdminIdentity(r *http.Request) bool {
	adminNpubsOnce.Do(func() {
		adminNpubs = map[string]bool{}
		for _, raw := range strings.Split(os.Getenv("PROXI_ADMIN_NPUBS"), ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if hexID, err := npubToXOnlyHex(raw); err == nil {
				adminNpubs[hexID] = true
			} else {
				adminNpubs[strings.ToLower(raw)] = true
			}
		}
	})
	if len(adminNpubs) == 0 {
		return false
	}
	for _, key := range []string{"userID", "npub"} {
		val, ok := r.Context().Value(key).(string)
		if !ok || val == "" {
			continue
		}
		if hexID, err := npubToXOnlyHex(val); err == nil && adminNpubs[hexID] {
			return true
		}
		if adminNpubs[strings.ToLower(val)] {
			return true
		}
	}
	return false
}

// callerUserID returns the JWT user id (empty when auth is disabled).
func callerUserID(r *http.Request) string {
	if v, ok := r.Context().Value("userID").(string); ok {
		return v
	}
	return ""
}

// handleContactsGet — GET /api/contacts — список контактов (P7: только свои)
func (s *Server) handleContactsGet(w http.ResponseWriter, r *http.Request) {
	contacts, err := s.db.GetContactsForOwner(callerUserID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if contacts == nil {
		contacts = []store.Contact{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"contacts": contacts,
		"count":    len(contacts),
	})
}

// handleContactsSave — POST /api/contacts — добавить/обновить контакт
func (s *Server) handleContactsSave(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req store.Contact
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id (npub) is required")
		return
	}

	// P7: the contact belongs to the caller; VPN mesh changes are admin-only.
	req.OwnerUserID = callerUserID(r)
	if req.GrantVPNAccess && !isAdminIdentity(r) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "granting VPN access is admin-only (PROXI_ADMIN_NPUBS)")
		return
	}

	if err := s.db.SaveContact(req); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Sync with VPN Manager (admins only — the mesh is server-global).
	if isAdminIdentity(r) {
		if req.GrantVPNAccess {
			// Only add to WireGuard if they provided a PublicKey
			if req.PublicKey != "" {
				if err := vpnMgr.AddPeer(req.Name, req.PublicKey, req.Endpoint); err != nil {
					log.Printf("Error AddPeer: %v", err)
				}
			}
		} else {
			if len(req.PublicKey) >= 16 {
				if err := vpnMgr.RemovePeer(req.PublicKey[:16]); err != nil {
					log.Printf("Error RemovePeer: %v", err)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "saved",
	})
}

// handleContactsRemove — DELETE /api/contacts — удалить контакт
func (s *Server) handleContactsRemove(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		ID        string `json:"id"`
		PublicKey string `json:"publicKey"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}

	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id is required")
		return
	}

	if err := s.db.DeleteContactForOwner(req.ID, callerUserID(r)); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Clean up VPN peer if necessary (admins only — P7)
	if isAdminIdentity(r) && len(req.PublicKey) >= 16 {
		if err := vpnMgr.RemovePeer(req.PublicKey[:16]); err != nil {
			log.Printf("Error RemovePeer: %v", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "removed",
	})
}

// getDataDir returns the data directory path
func getDataDir() string {
	dir := "/tmp/unkillable-messenger"
	if d := os.Getenv("DATA_DIR"); d != "" {
		dir = d
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("Error MkdirAll: %v", err)
	}
	return dir
}

// randomHex generates random hex string
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n*2]
}

// ========== Channel Handlers ==========

// handleChannelsGet — GET /api/channels — список каналов
func (s *Server) handleChannelsGet(w http.ResponseWriter, r *http.Request) {
	channels, err := s.db.GetChannels()
	if err != nil {
		log.Printf("ERROR: s.db.GetChannels: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to load channels")
		return
	}
	if channels == nil {
		channels = []store.Channel{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"channels": channels,
		"count":    len(channels),
	})
}

// handleChannelsPost — POST /api/channels — создать канал
func (s *Server) handleChannelsPost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Creator     string `json:"creator"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Channel name is required")
		return
	}
	if len(req.Name) > 100 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Channel name too long (max 100 chars)")
		return
	}
	if req.Creator == "" {
		req.Creator = "anonymous"
	}

	ch := store.Channel{
		ID:          fmt.Sprintf("ch-%d", time.Now().UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Creator:     req.Creator,
		Subscribers: 1,
		CreatedAt:   time.Now().Unix(),
	}

	if err := s.db.SaveChannel(ch); err != nil {
		log.Printf("ERROR: s.db.SaveChannel: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to save channel")
		return
	}

	writeJSON(w, http.StatusCreated, ch)
	log.Printf("📡 Channel created: %s by %s", req.Name, truncate(req.Creator, 16)+"...")
}

// handleChannelSubscribe — POST /api/channels/subscribe {channelId}
func (s *Server) handleChannelSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		ChannelID string `json:"channelId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if req.ChannelID == "" {
		writeError(w, 400, "BAD_REQUEST", "channelId required")
		return
	}
	if err := s.db.SubscribeChannel(req.ChannelID); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "subscribed", "channelId": req.ChannelID})
}
