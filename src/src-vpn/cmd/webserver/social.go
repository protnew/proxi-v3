package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

// ==================== Reactions ====================

// handleReactions handles POST/GET/DELETE for message reactions.
// POST:   {messageId, userNpub, emoji} — add reaction
// DELETE: {messageId, userNpub} — remove reaction
// GET:    ?messageId=... — get reactions for a message
func (s *Server) handleReactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.handleReactionAdd(w, r)
	case "DELETE":
		s.handleReactionRemove(w, r)
	case "GET":
		s.handleReactionGet(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET, POST or DELETE")
	}
}

func (s *Server) handleReactionAdd(w http.ResponseWriter, r *http.Request) {
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

	if err := s.db.AddReaction(req.MessageID, req.UserNpub, req.Emoji); err != nil {
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

func (s *Server) handleReactionRemove(w http.ResponseWriter, r *http.Request) {
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

	if err := s.db.RemoveReaction(req.MessageID, req.UserNpub); err != nil {
		log.Printf("ERROR: RemoveReaction: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to remove reaction")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "removed",
		"messageId": req.MessageID,
	})
}

func (s *Server) handleReactionGet(w http.ResponseWriter, r *http.Request) {
	messageID := r.URL.Query().Get("messageId")
	if strings.TrimSpace(messageID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "messageId query param is required")
		return
	}

	reactions, err := s.db.GetReactions(messageID)
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
func (s *Server) handleReadReceipts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.handleMarkRead(w, r)
	case "GET":
		s.handleGetReadReceipts(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
	}
}

func (s *Server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
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
		if err := s.db.MarkAllRead(req.UserNpub, req.BeforeTimestamp); err != nil {
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

	if err := s.db.MarkRead(req.MessageID, req.UserNpub); err != nil {
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

func (s *Server) handleGetReadReceipts(w http.ResponseWriter, r *http.Request) {
	messageID := r.URL.Query().Get("messageId")
	if strings.TrimSpace(messageID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "messageId query param is required")
		return
	}

	npubs, err := s.db.GetReadReceipts(messageID)
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
func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.handleProfileSave(w, r)
	case "GET":
		s.handleProfileGet(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
	}
}

func (s *Server) handleProfileSave(w http.ResponseWriter, r *http.Request) {
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

	if err := s.db.SaveProfile(req.Npub, req.DisplayName, req.AvatarURL, req.Bio); err != nil {
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

func (s *Server) handleProfileGet(w http.ResponseWriter, r *http.Request) {
	// Search mode
	if search := r.URL.Query().Get("search"); search != "" {
		profiles, err := s.db.SearchProfiles(search)
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

	profile, err := s.db.GetProfile(npub)
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
