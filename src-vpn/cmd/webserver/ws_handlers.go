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

	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/identity"
	"github.com/unkillable-messenger/vpn/store"

	"nhooyr.io/websocket"
)

var hub *chat.ChatHub

// handleWS upgrades HTTP to WebSocket and registers client
func handleWS(w http.ResponseWriter, r *http.Request) {
	userId := r.URL.Query().Get("userId")
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

	client := &chat.Client{
		UserID: userId,
		Conn:   conn,
		Send:   make(chan []byte, 64),
	}

	log.Printf("🔌 WS connected: %s", userId)
	client.Serve(r.Context())
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
	}
	log.Println("💬 Chat Hub initialized")
}

// ========== Peer Management Handlers ==========

// handlePeersGet — GET /api/peers — список пиров
func handlePeersGet(w http.ResponseWriter, r *http.Request) {
	status := vpnMgr.GetStatus()
	peers := status.Peers
	if peers == nil {
		peers = []vpn.Peer{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

// handlePeersAdd — POST /api/peers — добавить пира
func handlePeersAdd(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		Name      string `json:"name"`
		PublicKey string `json:"publicKey"`
		Endpoint  string `json:"endpoint"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}

	// Validation
	if strings.TrimSpace(req.PublicKey) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Public key is required")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Peer"
	}

	if err := vpnMgr.AddPeer(req.Name, req.PublicKey, req.Endpoint); err != nil {
		writeError(w, http.StatusInternalServerError, "ADD_PEER_ERROR", err.Error())
		return
	}

	// Return updated peer list
	status := vpnMgr.GetStatus()
	peers := status.Peers
	if peers == nil {
		peers = []vpn.Peer{}
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "added",
		"peers":  peers,
		"count":  len(peers),
	})

	log.Printf("👤 Peer added: %s (%s)", req.Name, truncate(req.PublicKey, 16)+"...")
}

// handlePeersRemove — DELETE /api/peers — удалить пира
func handlePeersRemove(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		PeerID string `json:"peerId"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}

	if strings.TrimSpace(req.PeerID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "peerId is required")
		return
	}

	if err := vpnMgr.RemovePeer(req.PeerID); err != nil {
		writeError(w, http.StatusInternalServerError, "REMOVE_PEER_ERROR", err.Error())
		return
	}

	// Return updated peer list
	status := vpnMgr.GetStatus()
	peers := status.Peers
	if peers == nil {
		peers = []vpn.Peer{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "removed",
		"peers":  peers,
		"count":  len(peers),
	})

	log.Printf("👤 Peer removed: %s", req.PeerID)
}

// getDataDir returns the data directory path
func getDataDir() string {
	dir := "/tmp/unkillable-messenger"
	if d := os.Getenv("DATA_DIR"); d != "" {
		dir = d
	}
	_ = os.MkdirAll(dir, 0700)
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
