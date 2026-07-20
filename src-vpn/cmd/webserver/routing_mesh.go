// File: routing_mesh.go
// P2-1 RESCUE 20260720: extracted from routing.go (God Object split).

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"time"
	"github.com/unkillable-messenger/vpn/mesh"

	"github.com/unkillable-messenger/vpn/nat"

	"github.com/unkillable-messenger/vpn/store"
	"github.com/unkillable-messenger/vpn/stream"

)

func (s *Server) handleIPFSUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST with multipart form")
		return
	}

	if !ipfsClient.IsAvailable() {
		writeError(w, 503, "IPFS_UNAVAILABLE", "IPFS daemon not running")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "BAD_REQUEST", "No file provided")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, 500, "READ_ERROR", err.Error())
		return
	}

	result, err := ipfsClient.UploadFile(header.Filename, data)
	if err != nil {
		writeError(w, 500, "UPLOAD_ERROR", err.Error())
		return
	}

	// Pin the file
	ipfsClient.PinFile(result.CID)

	writeJSON(w, 200, map[string]interface{}{
		"cid":        result.CID,
		"gatewayUrl": result.GatewayURL,
		"size":       result.Size,
		"filename":   header.Filename,
	})
}

// handleIPFSStatus returns IPFS daemon status.


func (s *Server) handleIPFSStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"available": ipfsClient.IsAvailable(),
	})
}

// handleNATDiscover discovers public IP and NAT type via STUN.


func (s *Server) handleNATDiscover(w http.ResponseWriter, r *http.Request) {
	result, err := nat.DiscoverPublicAddr("")
	if err != nil {
		writeError(w, 500, "STUN_ERROR", err.Error())
		return
	}
	natType, _ := nat.DetectNATType("")
	result.NATType = natType
	writeJSON(w, 200, result)
}

// handleTorStatus returns Tor SOCKS5 proxy status.


func (s *Server) handleTorStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"available": torDialer.IsTorRunning(),
		"proxy":     "127.0.0.1:9050",
	})
}

// ==================== Mesh Network Handlers ====================

// handleMeshPeers — GET /api/mesh/peers


func (s *Server) handleMeshPeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	peers := meshNet.GetAllPeers()
	if peers == nil {
		peers = []mesh.PeerInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

// handleMeshStats — GET /api/mesh/stats


func (s *Server) handleMeshStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	writeJSON(w, http.StatusOK, meshNet.GetStats())
}

// handleMeshAdd — POST /api/mesh/add


func (s *Server) handleMeshAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var peer mesh.PeerInfo
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if peer.ID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "peer id is required")
		return
	}
	meshNet.AddPeer(peer)
	log.Printf("🕸️  Mesh peer added: %s (%s)", peer.ID, peer.Address)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "added",
		"id":     peer.ID,
	})
}

// ==================== Stream Handlers (Sprint 3 — Task 1) ====================

// handleStreamCreate — POST /api/stream/create


func (s *Server) handleStreamCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		ChannelName string `json:"channelName"`
		StreamerID  string `json:"streamerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.StreamerID == "" {
		req.StreamerID = "anonymous"
	}
	if req.ChannelName == "" {
		req.ChannelName = "Live Stream"
	}
	streamObj := streamMgr.CreateStream(req.ChannelName, req.StreamerID)
	writeJSON(w, 201, streamObj)
	log.Printf("📺 Stream created: %s by %s (%s)", streamObj.ID, req.StreamerID, req.ChannelName)
}

// handleStreamList — GET /api/stream/list


func (s *Server) handleStreamList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use GET")
		return
	}
	streams := streamMgr.ListStreams()
	if streams == nil {
		streams = []stream.Stream{}
	}
	writeJSON(w, 200, map[string]interface{}{
		"streams": streams,
		"count":   len(streams),
	})
}

// handleStreamEnd — POST /api/stream/end


func (s *Server) handleStreamEnd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		StreamID string `json:"streamId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.StreamID == "" {
		writeError(w, 400, "BAD_REQUEST", "streamId required")
		return
	}
	if err := streamMgr.EndStream(req.StreamID); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "ended", "streamId": req.StreamID})
	log.Printf("📺 Stream ended: %s", req.StreamID)
}

// handleStreamSubscribe — POST /api/stream/subscribe


func (s *Server) handleStreamSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		StreamID string `json:"streamId"`
		UserID   string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.StreamID == "" {
		writeError(w, 400, "BAD_REQUEST", "streamId required")
		return
	}
	if err := streamMgr.Subscribe(req.StreamID, req.UserID); err != nil {
		writeError(w, 404, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "subscribed", "streamId": req.StreamID})
}

// ==================== Bot Handlers (Sprint 3 — Task 2) ====================

// handleBotRegister — POST /api/bots/register


func (s *Server) handleFederationPeerAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "url is required")
		return
	}

	// Add to in-memory federation relay
	if err := fedRelay.AddPeer(req.URL); err != nil {
		writeError(w, http.StatusConflict, "DUPLICATE", err.Error())
		return
	}

	// Persist to DB
	peer := store.FederationPeer{
		ID:       fmt.Sprintf("fp-%d", time.Now().UnixNano()),
		URL:      req.URL,
		LastSync: 0,
		Status:   "active",
	}
	if err := s.db.SaveFederationPeer(peer); err != nil {
		log.Printf("⚠️  Failed to persist federation peer: %v", err)
	}

	log.Printf("🌐 Federation peer added: %s", req.URL)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "added",
		"url":    req.URL,
	})
}

// handleFederationPeerRemove — DELETE /api/federation/peer


func (s *Server) handleFederationPeerRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use DELETE")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "url is required")
		return
	}

	// Remove from in-memory relay
	if err := fedRelay.RemovePeer(req.URL); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	// Remove from DB
	if err := s.db.DeleteFederationPeer(req.URL); err != nil {
		log.Printf("⚠️  Failed to delete federation peer from DB: %v", err)
	}

	log.Printf("🌐 Federation peer removed: %s", req.URL)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "removed",
		"url":    req.URL,
	})
}

// handleFederationPeerList — GET /api/federation/peer


func (s *Server) handleFederationPeerList(w http.ResponseWriter, r *http.Request) {
	peers, err := s.db.GetFederationPeers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if peers == nil {
		peers = []store.FederationPeer{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

// handleFederationSync — POST /api/federation/sync


func (s *Server) handleFederationSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		Since int64 `json:"since"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default: sync from 0 (full sync)
		req.Since = 0
	}

	if err := fedRelay.SyncEvents(context.Background(), req.Since); err != nil {
		writeError(w, http.StatusInternalServerError, "SYNC_ERROR", err.Error())
		return
	}

	// Update last_sync for all peers in DB
	now := time.Now().Unix()
	peers := fedRelay.GetPeers()
	for _, p := range peers {
		if err := s.db.UpdateFederationPeerSync(p, now); err != nil {
			log.Printf("⚠️  Failed to update sync time for %s: %v", p, err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "synced",
		"peerCount": len(peers),
		"syncedAt":  now,
	})
}


