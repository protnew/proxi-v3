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
	"time"


	"github.com/unkillable-messenger/vpn/bot"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/identity"
	"github.com/unkillable-messenger/vpn/store"

	"nhooyr.io/websocket"
)

var hub *chat.ChatHub

// handleWS upgrades HTTP to WebSocket and registers client
func handleWS(w http.ResponseWriter, r *http.Request) {
	userId := ""
	tokenStr := r.URL.Query().Get("token")
	if tokenStr != "" && globalAuthService != nil {
		if claims, err := globalAuthService.ValidateToken(tokenStr); err == nil {
			userId = claims.UserID
		}
	}
	
	if userId == "" {
		userId = r.URL.Query().Get("userId")
	}
	
	if userId == "" {
		userId = "anon-" + randomHex(4)
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("WS accept error: %v", err)
		return
	}

	log.Printf("🔌 WS connected: %s", userId)
	chat.ServeWS(hub, userId, conn, r.Context())
}

// handleIdentityGet returns current user's identity (generates if needed)
func handleIdentityGet(w http.ResponseWriter, r *http.Request) {
	// Try loading from DB first
	npub, nsec, seedPhrase, err := db.LoadIdentity()
	if err == nil && npub != "" {
		writeJSON(w, 200, map[string]interface{}{
			"npub":     npub,
			"nsec":     nsec,
			"isNew":    false,
			"mnemonic": seedPhrase,
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
	nsec = identity.PrivKeyToNsec(privKey)
	npub = identity.PubKeyToNpub(privKey.PubKey())

	// Save to SQLite
	if saveErr := db.SaveIdentity(npub, nsec, mnemonic); saveErr != nil {
		log.Printf("WARNING: failed to save identity to db: %v", saveErr)
	}

	// Also save to legacy file for backward compat
	identityPath := getDataDir() + "/identity.json"
	if fileErr := identity.SaveIdentity(privKey, identityPath); fileErr != nil {
		log.Printf("WARNING: failed to save identity file: %v", fileErr)
	}

	writeJSON(w, 200, map[string]interface{}{
		"npub":     npub,
		"nsec":     nsec,
		"mnemonic": mnemonic,
		"isNew":    true,
	})
}

// handleOnlineUsers returns list of connected users
func handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	users := hub.OnlineUsers()
	if users == nil {
		users = []string{}
	}
	writeJSON(w, 200, map[string]interface{}{
		"users": users,
		"count": len(users),
	})
}

// initHub creates the chat hub
func initHub() {
	hub = chat.NewChatHub()
	hub.OnMessage = func(msg *chat.Message) {
		log.Printf("💬 [%s→%s]: %s", msg.From, msg.To, truncate(msg.Text, 50))

		// Process bot commands
		if msg.Type == "chat" && msg.Text != "" {
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

		// Save chat messages to SQLite
		if msg.Type == "chat" {
			msgID := fmt.Sprintf("msg-%d-%s", msg.Ts, randomHex(4))
			storeMsg := store.Message{
				ID:            msgID,
				From:          msg.From,
				To:            msg.To,
				Text:          msg.Text,
				Timestamp:     msg.Ts,
				ReplyTo:       msg.ReplyTo,
				ForwardedFrom: msg.ForwardedFrom,
				TTL:           msg.TTL,
			}
			// Enrich reply with preview text
			if msg.ReplyTo != "" {
				if orig, err := db.GetMessageByID(msg.ReplyTo); err == nil {
					msg.ReplyToText = orig.Text
					msg.ReplyToFrom = orig.From
					if len(orig.Text) > 80 {
						msg.ReplyToText = orig.Text[:80] + "..."
					}
				}
			}
			// Assign server-generated ID back to message for WS broadcast
			msg.ID = msgID
			if err := db.SaveMessage(storeMsg); err != nil {
				log.Printf("ERROR: save message: %v", err)
			}
		}
	}
	log.Println("💬 Chat Hub initialized")

	// Start self-destruct cleaner (runs every 30 seconds)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if db == nil {
				continue
			}
			if n, err := db.CleanExpiredMessages(); err == nil && n > 0 {
				log.Printf("💣 Self-destruct: cleaned %d expired messages", n)
			}
		}
	}()
}

// ========== Peer Management Handlers ==========

// handleContactsGet — GET /api/contacts — список контактов
func handleContactsGet(w http.ResponseWriter, r *http.Request) {
	contacts, err := db.GetContacts()
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
func handleContactsSave(w http.ResponseWriter, r *http.Request) {
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

	if err := db.SaveContact(req); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Sync with VPN Manager
	if req.GrantVPNAccess {
		// Only add to WireGuard if they provided a PublicKey
		if req.PublicKey != "" {
				if err := vpnMgr.AddPeer(req.Name, req.PublicKey, req.Endpoint); err != nil { log.Printf("Error AddPeer: %v", err) }
		}
	} else {
		if len(req.PublicKey) >= 16 {
				if err := vpnMgr.RemovePeer(req.PublicKey[:16]); err != nil { log.Printf("Error RemovePeer: %v", err) }
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "saved",
	})
}

// handleContactsRemove — DELETE /api/contacts — удалить контакт
func handleContactsRemove(w http.ResponseWriter, r *http.Request) {
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

	if err := db.DeleteContact(req.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Clean up VPN peer if necessary
	if len(req.PublicKey) >= 16 {
				if err := vpnMgr.RemovePeer(req.PublicKey[:16]); err != nil { log.Printf("Error RemovePeer: %v", err) }
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
	if err := os.MkdirAll(dir, 0700); err != nil { log.Printf("Error MkdirAll: %v", err) }
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
func handleChannelsGet(w http.ResponseWriter, r *http.Request) {
	channels, err := db.GetChannels()
	if err != nil {
		log.Printf("ERROR: db.GetChannels: %v", err)
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
func handleChannelsPost(w http.ResponseWriter, r *http.Request) {
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

	if err := db.SaveChannel(ch); err != nil {
		log.Printf("ERROR: db.SaveChannel: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to save channel")
		return
	}

	writeJSON(w, http.StatusCreated, ch)
	log.Printf("📡 Channel created: %s by %s", req.Name, truncate(req.Creator, 16)+"...")
}

// handleChannelSubscribe — POST /api/channels/subscribe {channelId}
func handleChannelSubscribe(w http.ResponseWriter, r *http.Request) {
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
	if err := db.SubscribeChannel(req.ChannelID); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "subscribed", "channelId": req.ChannelID})
}
