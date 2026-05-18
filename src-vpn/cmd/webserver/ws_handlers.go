package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/identity"

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
	identityPath := getDataDir() + "/identity.json"

	privKey, err := identity.LoadIdentity(identityPath)
	if err != nil {
		privKey, _, err = identity.GenerateKeyPair()
		if err != nil {
			writeError(w, 500, "KEYGEN_ERROR", "Failed to generate keys")
			return
		}

		if err := identity.SaveIdentity(privKey, identityPath); err != nil {
			log.Printf("WARNING: failed to save identity: %v", err)
		}

		mnemonic, _ := identity.GenerateMnemonic()
		nsec := identity.PrivKeyToNsec(privKey)
		npub := identity.PubKeyToNpub(privKey.PubKey())

		writeJSON(w, 200, map[string]interface{}{
			"npub":     npub,
			"nsec":     nsec,
			"mnemonic": mnemonic,
			"isNew":    true,
		})
		return
	}

	nsec := identity.PrivKeyToNsec(privKey)
	npub := identity.PubKeyToNpub(privKey.PubKey())

	writeJSON(w, 200, map[string]interface{}{
		"npub":  npub,
		"nsec":  nsec,
		"isNew": false,
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
