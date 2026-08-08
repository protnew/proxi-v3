package vpn

// web_push.go — SL-051 Web Push + Nostr notification hooks (Phase 2).
// Browser Push needs VAPID keys + service worker; here we expose config API surface.

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"sync"
)

type PushConfig struct {
	Enabled     bool   `json:"enabled"`
	VapidPublic string `json:"vapidPublic,omitempty"`
	Phase       string `json:"phase"`
	Note        string `json:"note"`
}

var (
	pushMu    sync.RWMutex
	pushCfg   = PushConfig{Enabled: false, Phase: "phase2", Note: "Web Push requires VAPID + SW; Nostr kind:1059 remains primary offline channel"}
	vapidPub  string
	vapidPriv string
)

func init() {
	if pub := os.Getenv("VAPID_PUBLIC_KEY"); pub != "" {
		pushMu.Lock()
		vapidPub = pub
		vapidPriv = os.Getenv("VAPID_PRIVATE_KEY")
		pushCfg.VapidPublic = pub
		pushCfg.Enabled = vapidPriv != ""
		pushMu.Unlock()
	}
}

// EnsureDevVAPID creates ephemeral VAPID-like keys for local dev (not production).
func EnsureDevVAPID() (pub string, err error) {
	pushMu.Lock()
	defer pushMu.Unlock()
	if vapidPub != "" {
		return vapidPub, nil
	}
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", err
	}
	vapidPub = base64.RawURLEncoding.EncodeToString(b)
	b2 := make([]byte, 32)
	if _, err = rand.Read(b2); err != nil {
		return "", err
	}
	vapidPriv = base64.RawURLEncoding.EncodeToString(b2)
	pushCfg.VapidPublic = vapidPub
	pushCfg.Enabled = true
	pushCfg.Note = "dev ephemeral VAPID — set VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY for production"
	return vapidPub, nil
}

func GetPushConfig() PushConfig {
	pushMu.RLock()
	defer pushMu.RUnlock()
	return pushCfg
}

// HandlePushConfig GET /api/push/config
func HandlePushConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Query().Get("dev") == "1" {
		_, _ = EnsureDevVAPID()
	}
	cfg := GetPushConfig()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

// HandlePushSubscribe POST /api/push/subscribe — stores subscription JSON (memory only Phase 2 stub)
var pushSubsMu sync.Mutex
var pushSubs []json.RawMessage

func HandlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	pushSubsMu.Lock()
	pushSubs = append(pushSubs, raw)
	if len(pushSubs) > 100 {
		pushSubs = pushSubs[len(pushSubs)-100:]
	}
	n := len(pushSubs)
	pushSubsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "stored": n, "phase": "phase2"})
}
