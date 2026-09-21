package main

import (
	"encoding/json"
	"net/http"

	"github.com/unkillable-messenger/vpn/crypto"
)

// N7: PreKey bundle distribution API for desktop X3DH multi-device.
//
// GET  /api/keys/prekey/{userID}  — fetch someone's PreKey bundle (for X3DH)
// POST /api/keys/prekey           — publish your own PreKey bundle

// PreKeyBundleRequest is the POST body for publishing a bundle.
type PreKeyBundleRequest struct {
	IdentityKey    []byte `json:"identity_key"`
	SignedPreKey   []byte `json:"signed_prekey"`
	Signature      []byte `json:"signature"`
	OneTimePreKey  []byte `json:"one_time_prekey"`
}

// handlePreKeyPublish stores the caller's PreKey bundle.
func (s *Server) handlePreKeyPublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// P2 (2026-09-19): идентичность только из JWT claims — X-User-ID spoofable.
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req PreKeyBundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.IdentityKey) == 0 || len(req.SignedPreKey) == 0 {
		http.Error(w, "identity_key and signed_prekey required", http.StatusBadRequest)
		return
	}
	// P2 (2026-09-21): only verified, public-key bundles may be stored. The
	// signature over signed_prekey must verify against identity_key, so the
	// publisher proves possession of the matching private key — a raw private
	// X25519 blob can no longer enter the table.
	if !crypto.VerifyPreKeyBundle(&crypto.PreKeyBundle{
		IdentityKey:  req.IdentityKey,
		SignedPreKey: req.SignedPreKey,
		Signature:    req.Signature,
	}) {
		http.Error(w, "invalid bundle: signature over signed_prekey must verify against identity_key (public keys only)", http.StatusBadRequest)
		return
	}

	err := s.db.StorePreKeyBundle(
		userID,
		req.IdentityKey,
		req.SignedPreKey,
		req.Signature,
		req.OneTimePreKey,
	)
	if err != nil {
		http.Error(w, "store error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"user_id": userID,
	})
}

// handlePreKeyGet fetches a user's PreKey bundle by user ID (npub).
func (s *Server) handlePreKeyGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract userID from query param or path
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		// Try path suffix
		path := r.URL.Path
		if len(path) > len("/api/keys/prekey/") {
			userID = path[len("/api/keys/prekey/"):]
		}
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	bundle, err := s.db.GetPreKeyBundle(userID)
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if bundle == nil {
		http.Error(w, "no prekey bundle found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":         userID,
		"identity_key":    bundle.IdentityKey,
		"signed_prekey":   bundle.SignedPreKey,
		"signature":       bundle.Signature,
		"one_time_prekey": bundle.OneTimePreKey,
	})
}
