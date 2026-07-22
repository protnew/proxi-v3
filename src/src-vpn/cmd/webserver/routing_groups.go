// File: routing_groups.go
// P2-1 RESCUE 20260720: extracted from routing.go (God Object split).

package main

import (

	"encoding/json"
	"fmt"

	"log"
	"net/http"

	"time"
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
		CreatorNpub string   `json:"creatorNpub"`
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
	if req.Name == "" || req.CreatorNpub == "" {
		writeError(w, 400, "BAD_REQUEST", "name and creatorNpub required")
		return
	}

	groupID := fmt.Sprintf("grp-%d", time.Now().UnixNano())

	// Add creator as admin
	if err := s.db.SaveGroupMember(store.GroupMember{
		GroupID:  groupID,
		UserNpub: req.CreatorNpub,
		Role:     "admin",
		JoinedAt: time.Now().Unix(),
	}); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	// Add members
	for _, m := range req.Members {
		if m == req.CreatorNpub {
			continue
		}
		s.db.SaveGroupMember(store.GroupMember{
			GroupID:  groupID,
			UserNpub: m,
			Role:     "member",
			JoinedAt: time.Now().Unix(),
		})
	}

	// Also create as a channel for message routing
	ch := store.Channel{
		ID:          groupID,
		Name:        req.Name,
		Description: "Group chat",
		Creator:     req.CreatorNpub,
		Subscribers: len(req.Members) + 1,
		CreatedAt:   time.Now().Unix(),
	}
	s.db.SaveChannel(ch)

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
	if req.GroupID == "" || req.AdminNpub == "" || req.TargetNpub == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId, adminNpub and targetNpub required")
		return
	}

	isAdmin, err := s.db.IsGroupAdmin(req.GroupID, req.AdminNpub)
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
	if req.GroupID == "" || req.AdminNpub == "" || req.TargetNpub == "" || req.NewRole == "" {
		writeError(w, 400, "BAD_REQUEST", "groupId, adminNpub, targetNpub and newRole required")
		return
	}
	if req.NewRole != "admin" && req.NewRole != "moderator" && req.NewRole != "member" {
		writeError(w, 400, "BAD_REQUEST", "newRole must be admin, moderator or member")
		return
	}

	isAdmin, err := s.db.IsGroupAdmin(req.GroupID, req.AdminNpub)
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


