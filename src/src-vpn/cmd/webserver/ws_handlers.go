package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/unkillable-messenger/vpn/bot"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/identity"

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
	noteQueryTokenDeprecated("/ws", r)
	tokenStr, tokErr := wsToken(r)
	if tokErr != nil {
		http.Error(w, `{"error":"UNAUTHORIZED","message":"bad subprotocol"}`, http.StatusUnauthorized)
		return
	}
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
		if claims.Npub != "" {
			userId = claims.Npub
		} else {
			userId = claims.UserID
		}
	} else {
		userId = r.URL.Query().Get("userId")
		if userId == "" {
			userId = "anon-" + randomHex(4)
		}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: acceptOriginPatterns(r),
		Subprotocols:   wsAcceptProtocols(r),
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
		// P13/P16: DR ratchet state itself is RAM-only (documented limitation
		// until the E2E model fork X2 is decided). The persisted marker makes
		// this fail CLOSED: after a restart sessions are marked lost and the
		// clients re-handshake instead of hitting dead dr1: ciphertext.
		s.drSessions.SetMarker(s.db)
	}
	s.hub.OnMessage = func(msg *chat.Message) {
		// P5: never log message bodies on the hot path (metadata only).
		log.Printf("💬 [%s→%s] %s", msg.From, msg.To, blindLogLine(msg))

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
			if err := saveIncomingChat(s.db, msg); err != nil {
				log.Printf("P5 refuse persist: %s from %s", err.Error(), truncate(msg.From, 16))
				return
			}
			if msg.ReplyTo != "" {
				if orig, err := s.db.GetMessageByID(msg.ReplyTo); err == nil {
					msg.ReplyToText = orig.Text
					msg.ReplyToFrom = orig.From
					if len(orig.Text) > 80 {
						msg.ReplyToText = orig.Text[:80] + "..."
					}
				}
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
