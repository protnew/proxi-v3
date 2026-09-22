// File: social_messages.go
// P2-2 RESCUE 20260720: extracted from social.go.
// Category: message search/edit/delete handlers.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/store"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
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

	// H1: caller scope = JWT npub only — ?npub= query fallback allowed
	// searching another user's messages (IDOR). No npub → anonymous scope.
	npub, _ := r.Context().Value("npub").(string)
	if npub == "" {
		npub = "anonymous"
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	msgs, err := s.db.SearchMessages(query, npub, limit)
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
func (s *Server) handleEditMessage(w http.ResponseWriter, r *http.Request) {
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

	if err := s.db.EditMessage(req.ID, req.Text, senderNpub); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}

	// Broadcast edit event to WS clients
	if s.hub != nil {
		msg := &chat.Message{
			Type: "message_edited",
			ID:   req.ID, // P15: clients key tombstones/edits off the id field
			From: senderNpub,
			Text: req.Text,
			Ts:   time.Now().Unix(),
		}
		encoded, _ := msg.Encode()
		s.hub.Broadcast(encoded, "")
	}

	writeJSON(w, 200, map[string]interface{}{"status": "edited", "id": req.ID})
}

// handleDeleteMessage — DELETE /api/messages/delete {id}
func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
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

	if err := s.db.DeleteMessage(req.ID, senderNpub); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}

	// Broadcast delete event to WS clients
	if s.hub != nil {
		msg := &chat.Message{
			Type: "message_deleted",
			ID:   req.ID, // P15: id as a field, not smuggled through Text
			From: senderNpub,
			Ts:   time.Now().Unix(),
		}
		encoded, _ := msg.Encode()
		s.hub.Broadcast(encoded, "")
	}

	writeJSON(w, 200, map[string]interface{}{"status": "deleted", "id": req.ID})
}
