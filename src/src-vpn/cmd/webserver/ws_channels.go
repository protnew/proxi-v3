package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

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
