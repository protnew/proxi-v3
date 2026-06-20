package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/store"
)

// ==================== Reactions ====================

// handleReactions handles POST/GET/DELETE for message reactions.
// POST:   {messageId, userNpub, emoji} — add reaction
// DELETE: {messageId, userNpub} — remove reaction
// GET:    ?messageId=... — get reactions for a message
func handleReactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		handleReactionAdd(w, r)
	case "DELETE":
		handleReactionRemove(w, r)
	case "GET":
		handleReactionGet(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET, POST or DELETE")
	}
}

func handleReactionAdd(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		MessageID string `json:"messageId"`
		UserNpub  string `json:"userNpub"`
		Emoji     string `json:"emoji"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}

	if userNpub, ok := r.Context().Value("npub").(string); ok && userNpub != "" {
		req.UserNpub = userNpub
	}

	if strings.TrimSpace(req.MessageID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "messageId is required")
		return
	}
	if strings.TrimSpace(req.UserNpub) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "userNpub is required")
		return
	}
	if strings.TrimSpace(req.Emoji) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "emoji is required")
		return
	}
	if len(req.Emoji) > 10 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "emoji too long")
		return
	}

	if err := db.AddReaction(req.MessageID, req.UserNpub, req.Emoji); err != nil {
		log.Printf("ERROR: AddReaction: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to add reaction")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "added",
		"messageId": req.MessageID,
		"userNpub":  req.UserNpub,
		"emoji":     req.Emoji,
	})
}

func handleReactionRemove(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		MessageID string `json:"messageId"`
		UserNpub  string `json:"userNpub"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}

	if userNpub, ok := r.Context().Value("npub").(string); ok && userNpub != "" {
		req.UserNpub = userNpub
	}

	if strings.TrimSpace(req.MessageID) == "" || strings.TrimSpace(req.UserNpub) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "messageId and userNpub are required")
		return
	}

	if err := db.RemoveReaction(req.MessageID, req.UserNpub); err != nil {
		log.Printf("ERROR: RemoveReaction: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to remove reaction")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "removed",
		"messageId": req.MessageID,
	})
}

func handleReactionGet(w http.ResponseWriter, r *http.Request) {
	messageID := r.URL.Query().Get("messageId")
	if strings.TrimSpace(messageID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "messageId query param is required")
		return
	}

	reactions, err := db.GetReactions(messageID)
	if err != nil {
		log.Printf("ERROR: GetReactions: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to get reactions")
		return
	}
	if reactions == nil {
		reactions = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reactions": reactions,
		"count":     len(reactions),
	})
}

// ==================== Read Receipts ====================

// handleReadReceipts handles POST/GET for read receipts.
// POST:  {messageId, userNpub} — mark message as read
// GET:   ?messageId=... — get read receipts for a message
// POST /mark-all: {userNpub, beforeTimestamp} — mark all as read
func handleReadReceipts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		handleMarkRead(w, r)
	case "GET":
		handleGetReadReceipts(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
	}
}

func handleMarkRead(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		MessageID       string `json:"messageId"`
		UserNpub        string `json:"userNpub"`
		BeforeTimestamp int64  `json:"beforeTimestamp"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}

	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	
	if userNpub, ok := r.Context().Value("npub").(string); ok && userNpub != "" {
		req.UserNpub = userNpub
	}

	if strings.TrimSpace(req.UserNpub) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "userNpub is required")
		return
	}

	// If beforeTimestamp is set, mark all messages as read
	if req.BeforeTimestamp > 0 {
		if err := db.MarkAllRead(req.UserNpub, req.BeforeTimestamp); err != nil {
			log.Printf("ERROR: MarkAllRead: %v", err)
			writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to mark all as read")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "all_read",
			"userNpub": req.UserNpub,
		})
		return
	}

	// Single message mark read
	if strings.TrimSpace(req.MessageID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "messageId is required (or use beforeTimestamp)")
		return
	}

	if err := db.MarkRead(req.MessageID, req.UserNpub); err != nil {
		log.Printf("ERROR: MarkRead: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to mark as read")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "read",
		"messageId": req.MessageID,
		"userNpub":  req.UserNpub,
	})
}

func handleGetReadReceipts(w http.ResponseWriter, r *http.Request) {
	messageID := r.URL.Query().Get("messageId")
	if strings.TrimSpace(messageID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "messageId query param is required")
		return
	}

	npubs, err := db.GetReadReceipts(messageID)
	if err != nil {
		log.Printf("ERROR: GetReadReceipts: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to get read receipts")
		return
	}
	if npubs == nil {
		npubs = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messageId": messageID,
		"readBy":    npubs,
		"count":     len(npubs),
	})
}

// ==================== User Profiles ====================

// handleProfiles handles GET/POST for user profiles.
// POST: {npub, displayName, avatarUrl, bio} — save profile
// GET:  ?npub=... — get single profile
// GET:  ?search=... — search profiles
func handleProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		handleProfileSave(w, r)
	case "GET":
		handleProfileGet(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
	}
}

func handleProfileSave(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		Npub        string `json:"npub"`
		DisplayName string `json:"displayName"`
		AvatarURL   string `json:"avatarUrl"`
		Bio         string `json:"bio"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}

	if npub, ok := r.Context().Value("npub").(string); ok && npub != "" {
		req.Npub = npub
	}

	if strings.TrimSpace(req.Npub) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "npub is required")
		return
	}
	if len(req.DisplayName) > 100 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "displayName too long (max 100 chars)")
		return
	}
	if len(req.Bio) > 500 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "bio too long (max 500 chars)")
		return
	}

	if err := db.SaveProfile(req.Npub, req.DisplayName, req.AvatarURL, req.Bio); err != nil {
		log.Printf("ERROR: SaveProfile: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to save profile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "saved",
		"npub":        req.Npub,
		"displayName": req.DisplayName,
		"avatarUrl":   req.AvatarURL,
		"bio":         req.Bio,
	})

	log.Printf("👤 Profile saved: %s (%s)", req.DisplayName, truncate(req.Npub, 16)+"...")
}

func handleProfileGet(w http.ResponseWriter, r *http.Request) {
	// Search mode
	if search := r.URL.Query().Get("search"); search != "" {
		profiles, err := db.SearchProfiles(search)
		if err != nil {
			log.Printf("ERROR: SearchProfiles: %v", err)
			writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to search profiles")
			return
		}
		if profiles == nil {
			profiles = []map[string]interface{}{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"profiles": profiles,
			"count":    len(profiles),
		})
		return
	}

	// Single profile lookup
	npub := r.URL.Query().Get("npub")
	if strings.TrimSpace(npub) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "npub or search query param is required")
		return
	}

	profile, err := db.GetProfile(npub)
	if err != nil {
		// Return empty profile if not found
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"npub":        npub,
			"displayName": "",
			"avatarUrl":   "",
			"bio":         "",
			"found":       false,
		})
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// ==================== Search ====================

// handleSearch handles GET /api/search — search messages.
// Query params: q (search term), npub (user npub), limit (max results)
func handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}

	query := r.URL.Query().Get("q")
	if strings.TrimSpace(query) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "q query param is required")
		return
	}
	if len(query) > 200 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Search query too long (max 200 chars)")
		return
	}

	npub := r.URL.Query().Get("npub")
	if npub == "" {
		npub = "anonymous"
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	msgs, err := db.SearchMessages(query, npub, limit)
	if err != nil {
		log.Printf("ERROR: SearchMessages: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to search messages")
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": msgs,
		"count":   len(msgs),
		"query":   query,
		"time":    fmt.Sprintf("%dms", time.Now().Unix()%1000),
	})
}

// ==================== Edit/Delete Messages ====================

// handleEditMessage — PUT /api/messages/edit {id, text}
func handleEditMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use PUT or POST")
		return
	}
	var req struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.ID == "" || req.Text == "" {
		writeError(w, 400, "BAD_REQUEST", "id and text required")
		return
	}
	
	senderNpub := ""
	if npub, ok := r.Context().Value("npub").(string); ok {
		senderNpub = npub
	}
	
	if err := db.EditMessage(req.ID, req.Text, senderNpub); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}

	// Broadcast edit event to WS clients
	if hub != nil {
		msg := &chat.Message{
			Type: "message_edited",
			Text: req.Text,
			Ts:   time.Now().Unix(),
		}
		msg.From = ""
		encoded, _ := msg.Encode()
		hub.Broadcast(encoded, "")
	}

	writeJSON(w, 200, map[string]interface{}{"status": "edited", "id": req.ID})
}

// handleDeleteMessage — DELETE /api/messages/delete {id}
func handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use DELETE or POST")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.ID == "" {
		writeError(w, 400, "BAD_REQUEST", "id required")
		return
	}
	
	senderNpub := ""
	if npub, ok := r.Context().Value("npub").(string); ok {
		senderNpub = npub
	}

	if err := db.DeleteMessage(req.ID, senderNpub); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}

	// Broadcast delete event to WS clients
	if hub != nil {
		msg := &chat.Message{
			Type: "message_deleted",
			Ts:   time.Now().Unix(),
		}
		msg.Text = req.ID
		encoded, _ := msg.Encode()
		hub.Broadcast(encoded, "")
	}

	writeJSON(w, 200, map[string]interface{}{"status": "deleted", "id": req.ID})
}

