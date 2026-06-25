package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/content"
	"github.com/unkillable-messenger/vpn/store"
)

var (
	pinMgr       *chat.PinManager
	slowModeMgr  *chat.SlowModeManager
	previewMgr   *chat.PreviewService
	killSwitch   *vpn.KillSwitch
	reconnectMgr *vpn.ReconnectManager
)

func initExtraRoutes(dbStore *store.Store, apiChain func(http.HandlerFunc) http.HandlerFunc) {
	// 1. Content Vault
	pipeline := content.NewUploadPipeline(1024*1024, 10, 14)
	vault := content.NewContentVault(pipeline, dbStore.DB())
	content.SetDefaultVault(vault)

	http.HandleFunc("/api/content/upload", apiChain(content.HandleUpload))
	http.HandleFunc("/api/content/list", apiChain(content.HandleList))
	http.HandleFunc("/api/content/", apiChain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			content.HandleDelete(w, r)
		} else {
			content.HandleDownload(w, r)
		}
	}))
	http.HandleFunc("/api/stream/", apiChain(content.HandleStream))

	// 2. Chat Pinning
	pinMgr = chat.NewPinManager()
	http.HandleFunc("/api/messages/pin", apiChain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
			return
		}
		var req struct {
			MsgID     string `json:"msgId"`
			ChannelID string `json:"channelId"`
			PinnedBy  string `json:"pinnedBy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		pinMgr.Pin(req.MsgID, req.ChannelID, req.PinnedBy)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	http.HandleFunc("/api/messages/pinned", apiChain(func(w http.ResponseWriter, r *http.Request) {
		ch := r.URL.Query().Get("channelId")
		pins := pinMgr.ListPinned(ch)
		writeJSON(w, http.StatusOK, pins)
	}))

	// 3. Chat SlowMode
	slowModeMgr = chat.NewSlowModeManager()

	// 4. URL Preview
	previewMgr = chat.NewPreviewService(24 * time.Hour)
	http.HandleFunc("/api/url-preview", apiChain(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Query().Get("url")
		if url == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "url required")
			return
		}
		res, err := previewMgr.FetchPreview(url)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "ERROR", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
	}))

	// 5. VPN KillSwitch & Reconnect
	killSwitch = vpn.NewKillSwitch("wg0")
	reconnectMgr = vpn.NewReconnectManager()
}
