package api

import (
	"encoding/json"
	"net/http"

	"github.com/unkillable-messenger/vpn/store"
)

// V2Handler implements JSON:API v2 endpoints.
type V2Handler struct {
	db *store.Store
}

// NewV2Handler creates a v2 API handler.
func NewV2Handler(db *store.Store) *V2Handler {
	return &V2Handler{db: db}
}

// JSONAPIResponse wraps a JSON:API response.
type JSONAPIResponse struct {
	Data  interface{}            `json:"data,omitempty"`
	Meta  map[string]interface{} `json:"meta,omitempty"`
	Error string                 `json:"error,omitempty"`
}

// HandleQuery processes a JSON:API query.
func (h *V2Handler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	w.Header().Set("Content-Type", "application/json")

	switch resource {
	case "messages":
		messages := h.getMessages(r.URL.Query().Get("channel"), 50)
		json.NewEncoder(w).Encode(JSONAPIResponse{Data: messages})
	case "channels":
		channels := h.getChannels()
		json.NewEncoder(w).Encode(JSONAPIResponse{Data: channels})
	case "peers":
		peers := h.getPeers()
		json.NewEncoder(w).Encode(JSONAPIResponse{Data: peers})
	default:
		json.NewEncoder(w).Encode(JSONAPIResponse{Error: "unknown resource: " + resource})
	}
}

func (h *V2Handler) getMessages(channel string, limit int) []map[string]interface{} {
	d := h.db.DB()
	query := "SELECT id, sender, text, timestamp FROM messages"
	args := []interface{}{}
	if channel != "" {
		query += " WHERE recipient = ?"
		args = append(args, channel)
	}
	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)
	rows, err := d.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var id, sender, text string
		var ts int64
		rows.Scan(&id, &sender, &text, &ts)
		result = append(result, map[string]interface{}{
			"type": "message", "id": id,
			"attributes": map[string]interface{}{"sender": sender, "text": text, "timestamp": ts},
		})
	}
	return result
}

func (h *V2Handler) getChannels() []map[string]interface{} {
	d := h.db.DB()
	rows, err := d.Query("SELECT id, name, creator FROM channels")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var id, name, creator string
		rows.Scan(&id, &name, &creator)
		result = append(result, map[string]interface{}{
			"type": "channel", "id": id,
			"attributes": map[string]interface{}{"name": name, "creator": creator},
		})
	}
	return result
}

func (h *V2Handler) getPeers() []map[string]interface{} {
	d := h.db.DB()
	rows, err := d.Query("SELECT id, name FROM peers")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var id, name string
		rows.Scan(&id, &name)
		result = append(result, map[string]interface{}{
			"type": "peer", "id": id,
			"attributes": map[string]interface{}{"name": name},
		})
	}
	return result
}
