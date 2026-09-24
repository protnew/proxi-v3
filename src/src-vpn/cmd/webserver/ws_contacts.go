package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/unkillable-messenger/vpn/store"
)

// handleContactsGet — GET /api/contacts — список контактов (P7: только свои)
func (s *Server) handleContactsGet(w http.ResponseWriter, r *http.Request) {
	contacts, err := s.db.GetContactsForOwner(callerUserID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if contacts == nil {
		contacts = []store.Contact{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"contacts": contacts,
		"count":    len(contacts),
	})
}

// handleContactsSave — POST /api/contacts — добавить/обновить контакт
func (s *Server) handleContactsSave(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req store.Contact
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id (npub) is required")
		return
	}

	// P7: the contact belongs to the caller; VPN mesh changes are admin-only.
	req.OwnerUserID = callerUserID(r)
	if req.GrantVPNAccess && !isAdminIdentity(r) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "granting VPN access is admin-only (PROXI_ADMIN_NPUBS)")
		return
	}

	if err := s.db.SaveContact(req); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Sync with VPN Manager (admins only — the mesh is server-global).
	if isAdminIdentity(r) {
		if req.GrantVPNAccess {
			// Only add to WireGuard if they provided a PublicKey
			if req.PublicKey != "" {
				if err := vpnMgr.AddPeer(req.Name, req.PublicKey, req.Endpoint); err != nil {
					log.Printf("Error AddPeer: %v", err)
				}
			}
		} else {
			if len(req.PublicKey) >= 16 {
				if err := vpnMgr.RemovePeer(req.PublicKey[:16]); err != nil {
					log.Printf("Error RemovePeer: %v", err)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "saved",
	})
}

// handleContactsRemove — DELETE /api/contacts — удалить контакт
func (s *Server) handleContactsRemove(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		ID        string `json:"id"`
		PublicKey string `json:"publicKey"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}

	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id is required")
		return
	}

	if err := s.db.DeleteContactForOwner(req.ID, callerUserID(r)); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Clean up VPN peer if necessary (admins only — P7)
	if isAdminIdentity(r) && len(req.PublicKey) >= 16 {
		if err := vpnMgr.RemovePeer(req.PublicKey[:16]); err != nil {
			log.Printf("Error RemovePeer: %v", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "removed",
	})
}
