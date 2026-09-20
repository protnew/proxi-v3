package main

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/crypto"
)

// handleSessionEstablish — POST /api/keys/session
// CRYP-011: establishes Double Ratchet session via X3DH.
//
// Modes:
//   - "bootstrap_pair": server generates ephemeral X3DH material for both sides
//     and stores both ratchet states (dev/test + local desktop ceremony).
//   - Production PWA continues to use client-side NIP-44; this path is for
//     Go/desktop multi-device X3DH+DR wiring.
func (s *Server) handleSessionEstablish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	if s.drSessions == nil {
		s.drSessions = chat.NewDRSessionStore()
	}
	if s.db != nil {
		s.drSessions.SetMarker(s.db) // P13
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, 400, "READ_ERROR", "bad body")
		return
	}
	defer r.Body.Close()

	var req struct {
		LocalNpub  string `json:"local_npub"`
		RemoteNpub string `json:"remote_npub"`
		Mode       string `json:"mode"` // bootstrap_pair
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, 400, "PARSE_ERROR", "invalid JSON")
		return
	}
	if req.LocalNpub == "" || req.RemoteNpub == "" {
		writeError(w, 400, "VALIDATION_ERROR", "local_npub and remote_npub required")
		return
	}
	if req.Mode == "" {
		req.Mode = "bootstrap_pair"
	}

	switch req.Mode {
	case "bootstrap_pair":
		bobMat, err := crypto.NewX3DHBobMaterial()
		if err != nil {
			writeError(w, 500, "X3DH_ERROR", err.Error())
			return
		}
		aliceIK, err := crypto.GenerateX3DHKeyPair()
		if err != nil {
			writeError(w, 500, "X3DH_ERROR", err.Error())
			return
		}
		// CRYP-011: X3DH.Initiate path inside BootstrapDoubleRatchet
		if err := s.drSessions.BootstrapPair(req.LocalNpub, req.RemoteNpub, aliceIK, bobMat); err != nil {
			writeError(w, 500, "SESSION_ERROR", err.Error())
			return
		}
		log.Printf("CRYP-011: X3DH+DR session established %s ↔ %s", truncate(req.LocalNpub, 12), truncate(req.RemoteNpub, 12))
		writeJSON(w, 201, map[string]interface{}{
			"status":      "established",
			"mode":        "bootstrap_pair",
			"local_npub":  req.LocalNpub,
			"remote_npub": req.RemoteNpub,
			"x3dh":        true,
			"double_ratchet": true,
			"alice_ik_pub": hex.EncodeToString(aliceIK.Pub[:]),
			"bob_ik_pub":   hex.EncodeToString(bobMat.IK.Pub[:]),
		})
	default:
		writeError(w, 400, "VALIDATION_ERROR", "unsupported mode")
	}
}

// handleSessionStatus — GET /api/keys/session?local=&remote=
func (s *Server) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	local := r.URL.Query().Get("local")
	remote := r.URL.Query().Get("remote")
	if local == "" || remote == "" {
		writeError(w, 400, "VALIDATION_ERROR", "local and remote query required")
		return
	}
	ok := s.drSessions != nil && s.drSessions.HasSession(local, remote)
	writeJSON(w, 200, map[string]interface{}{
		"local":  local,
		"remote": remote,
		"active": ok,
	})
}
