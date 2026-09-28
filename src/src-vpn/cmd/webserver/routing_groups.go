// File: routing_groups.go
// P2-1 RESCUE 20260720: extracted from routing.go (God Object split).

package main

import (

	"encoding/json"
	"log"
	"net/http"

	"github.com/unkillable-messenger/vpn/store"

)

func (s *Server) handleGroupList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	channels, err := s.db.GetChannels()
	if err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}
	groups := make([]map[string]interface{}, 0)
	for _, ch := range channels {
		groups = append(groups, map[string]interface{}{
			"id":        ch.ID,
			"name":      ch.Name,
			"creator":   ch.Creator,
			"createdAt": ch.CreatedAt,
		})
	}
	writeJSON(w, 200, map[string]interface{}{
		"groups": groups,
		"count":  len(groups),
	})
}

// handleGroupCreate — POST /api/groups/create


func (s *Server) handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		Name        string   `json:"name"`
		CreatorNpub string   `json:"creatorNpub"` // ignored when auth is on (P4)
		Members     []string `json:"members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	// P4: the creator is whoever the JWT says — body claims are not trusted.
	creator := req.CreatorNpub
	if npub, ok := r.Context().Value("npub").(string); ok && npub != "" {
		creator = npub
	}
	if req.Name == "" || creator == "" {
		writeError(w, 400, "BAD_REQUEST", "name required")
		return
	}

	groupID, err := s.db.CreateGroup(req.Name, creator, req.Members)
	if err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"status":  "created",
		"groupId": groupID,
		"name":    req.Name,
		"members": len(req.Members) + 1,
	})
	log.Printf("👥 Group created: %s (%s) with %d members", req.Name, groupID, len(req.Members)+1)
}

// handleGroupMembers — GET /api/groups/members?groupId=...


func (s *Server) handleGroupMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	groupID := r.URL.Query().Get("groupId")
	if groupID == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId required")
		return
	}
	members, err := s.db.GetGroupMembers(groupID)
	if err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}
	// P4: the member list is visible only to participants.
	if npub, ok := r.Context().Value("npub").(string); ok && npub != "" {
		member := false
		for _, m := range members {
			if m.UserNpub == npub {
				member = true
				break
			}
		}
		if !member {
			writeError(w, 403, "FORBIDDEN", "not a group member")
			return
		}
	}
	if members == nil {
		members = []store.GroupMember{}
	}
	writeJSON(w, 200, map[string]interface{}{
		"groupId": groupID,
		"members": members,
		"count":   len(members),
	})
}

// handleGroupKick — DELETE /api/groups/kick (admin only)


func (s *Server) handleGroupKick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST or DELETE")
		return
	}
	var req struct {
		GroupID    string `json:"groupId"`
		AdminNpub  string `json:"adminNpub"`
		TargetNpub string `json:"targetNpub"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.GroupID == "" || req.TargetNpub == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId and targetNpub required")
		return
	}
	// P4: admin identity comes from JWT claims, never from the body.
	adminNpub := req.AdminNpub
	if npub, ok := r.Context().Value("npub").(string); ok && npub != "" {
		adminNpub = npub
	}
	if adminNpub == "" {
		writeError(w, 401, "UNAUTHORIZED", "identity required")
		return
	}

	isAdmin, err := s.db.IsGroupAdmin(req.GroupID, adminNpub)
	if err != nil || !isAdmin {
		writeError(w, 403, "FORBIDDEN", "Only admin can kick members")
		return
	}

	if err := s.db.RemoveGroupMember(req.GroupID, req.TargetNpub); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{"status": "kicked", "groupId": req.GroupID, "target": req.TargetNpub})
}

// handleGroupPromote — POST /api/groups/promote (admin only)


func (s *Server) handleGroupPromote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		GroupID    string `json:"groupId"`
		AdminNpub  string `json:"adminNpub"`
		TargetNpub string `json:"targetNpub"`
		NewRole    string `json:"newRole"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.GroupID == "" || req.TargetNpub == "" || req.NewRole == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId, targetNpub and newRole required")
		return
	}
	if req.NewRole != "admin" && req.NewRole != "moderator" && req.NewRole != "member" {
		writeError(w, 400, "BAD_REQUEST", "newRole must be admin, moderator or member")
		return
	}
	// P4: admin identity comes from JWT claims, never from the body.
	adminNpub := req.AdminNpub
	if npub, ok := r.Context().Value("npub").(string); ok && npub != "" {
		adminNpub = npub
	}
	if adminNpub == "" {
		writeError(w, 401, "UNAUTHORIZED", "identity required")
		return
	}

	isAdmin, err := s.db.IsGroupAdmin(req.GroupID, adminNpub)
	if err != nil || !isAdmin {
		writeError(w, 403, "FORBIDDEN", "Only admin can promote members")
		return
	}

	if err := s.db.UpdateGroupMemberRole(req.GroupID, req.TargetNpub, req.NewRole); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{"status": "promoted", "newRole": req.NewRole})
}

// handleSplitTunnel — POST /api/vpn/split-tunnel


