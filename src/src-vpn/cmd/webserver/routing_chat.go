// File: routing_chat.go
// P2-1 RESCUE 20260720: extracted from routing.go (God Object split).

package main

import (
	vpnroot "github.com/unkillable-messenger/vpn"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/crypto"

	"github.com/unkillable-messenger/vpn/store"

	"go.uber.org/zap"

)

func (s *Server) handleMessagesGet(w http.ResponseWriter, r *http.Request) {
	var since int64 = 0
	if v := r.URL.Query().Get("since"); v != "" {
		fmt.Sscanf(v, "%d", &since)
	}
	// Prefer JWT context npub; accept query npub/peer for filters
	npub, _ := r.Context().Value("npub").(string)
	if npub == "" {
		npub = r.URL.Query().Get("npub")
	}
	peer := r.URL.Query().Get("peer")
	if peer == "" {
		peer = r.URL.Query().Get("with")
	}
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}

	msgs, err := s.db.GetMessages(limit, since, npub)
	if err != nil {
		log.Printf("ERROR: s.db.GetMessages: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to load messages")
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}

	// MSG-003: drop empty-text rows from API response
	filtered := make([]store.Message, 0, len(msgs))
	for _, m := range msgs {
		if strings.TrimSpace(m.Text) == "" {
			continue
		}
		// Optional conversation filter (peer = other party)
		if peer != "" {
			if !((m.From == npub && m.To == peer) || (m.From == peer && m.To == npub) || m.To == "broadcast") {
				continue
			}
		}
		filtered = append(filtered, m)
	}
	msgs = filtered

	// Auto-decrypt messages for the current user
	currentUserNpub := npub
	if currentUserNpub != "" {
		recipientBundle, err := s.db.GetPreKeyBundle(currentUserNpub)
		if err == nil && recipientBundle != nil {
			privBundle := &crypto.PreKeyBundle{
				IdentityKey: recipientBundle.IdentityKey,
			}
			for i, m := range msgs {
				if m.Encrypted && m.To == currentUserNpub {
					plaintext, err := chat.DecryptMessageFromSender(m.Text, privBundle.IdentityKey, m.From)
					if err == nil {
						msgs[i].Text = plaintext
						msgs[i].Encrypted = false
					} else {
						zap.S().Warnf("Failed to decrypt message %s from %s: %v", m.ID, m.From, err)
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": msgs,
		"count":    len(msgs),
	})
}


func (s *Server) handleMessagesPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST to send messages")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024)) // 64KB max
	if err != nil {
		writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Invalid JSON")
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}

	// Validation
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Message text is required")
		return
	}
	// SEC-002: single SoT — vpnroot.MaxMessageLen / ValidateMessage
	if err := vpnroot.ValidateMessage(req.Text); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.To == "" {
		req.To = "broadcast"
	}

	// Force sender from context (JWT token) to prevent spoofing
	if userNpub, ok := r.Context().Value("npub").(string); ok && userNpub != "" {
		req.From = userNpub
	} else if req.From == "" {
		req.From = "anonymous"
	}

	// Enable E2E Encryption
	encryptedText := req.Text
	isEncrypted := false
	if req.To != "broadcast" {
		// Fetch sender's prekey bundle
		bundleRow, err := s.db.GetPreKeyBundle(req.From)
		if err == nil && bundleRow != nil {
			senderBundle := &crypto.PreKeyBundle{
				IdentityKey: bundleRow.IdentityKey,
			}
			enc, err := chat.EncryptMessageForRecipient(req.Text, senderBundle.IdentityKey, req.To)
			if err == nil {
				encryptedText = enc
				isEncrypted = true
			} else {
				log.Printf("WARNING: E2E encryption failed: %v", err)
			}
		}
	}

	msg := store.Message{
		ID:        fmt.Sprintf("msg-%d-%s", time.Now().UnixNano(), req.From[:min(8, len(req.From))]),
		From:      req.From,
		To:        req.To,
		Text:      encryptedText,
		Encrypted: isEncrypted,
		Timestamp: time.Now().Unix(),
	}

	// Persist to SQLite
	if err := s.db.SaveMessage(msg); err != nil {
		log.Printf("WARNING: failed to save message to db: %v", err)
	}

	writeJSON(w, http.StatusCreated, msg)

	// Redact plaintext messages from logs (Compliance P0)
	log.Printf("💬 Message from %s to %s (%d bytes)", req.From, req.To, len(req.Text))
}


func (s *Server) handleScheduleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
		return
	}
	var req struct {
		Text      string `json:"text"`
		SendAt    int64  `json:"sendAt"`
		Recipient string `json:"recipient"`
		Sender    string `json:"sender"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "BAD_REQUEST", err.Error())
		return
	}
	if err := validate.Struct(req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.Text == "" || req.SendAt == 0 {
		writeError(w, 400, "BAD_REQUEST", "text and sendAt required")
		return
	}
	if req.SendAt <= time.Now().Unix() {
		writeError(w, 400, "BAD_REQUEST", "sendAt must be in the future")
		return
	}
	if req.Recipient == "" {
		req.Recipient = "broadcast"
	}
	if req.Sender == "" {
		req.Sender = "anonymous"
	}

	sm := store.ScheduledMessage{
		ID:        fmt.Sprintf("sched-%d-%s", time.Now().UnixNano(), randomHex(4)),
		Sender:    req.Sender,
		Recipient: req.Recipient,
		Text:      req.Text,
		SendAt:    req.SendAt,
		Status:    "pending",
		CreatedAt: time.Now().Unix(),
	}
	if err := s.db.SaveScheduledMessage(sm); err != nil {
		writeError(w, 500, "DB_ERROR", err.Error())
		return
	}

	sendAtTime := time.Unix(req.SendAt, 0).Format("02.01.2006 15:04")
	writeJSON(w, 200, map[string]interface{}{
		"status":     "scheduled",
		"id":         sm.ID,
		"sendAt":     req.SendAt,
		"sendAtTime": sendAtTime,
	})
	log.Printf("📅 Scheduled: %s → %s at %s", sm.ID, req.Recipient, sendAtTime)
}

// handleSwitchSetup — POST /api/switch/setup


func startDeadMansSwitchWorker(ctx context.Context, s *Server) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			switches, err := s.db.GetExpiredSwitches()
			if err != nil {
				zap.S().Errorf("Failed to check expired switches: %v", err)
				continue
			}

			for _, dms := range switches {
				zap.S().Infof("💀 Triggering Dead Man's Switch %s for user %s", dms.ID, dms.UserNpub)
				// Broadcast via hub if possible, or save as a system message to recipient
				msg := store.Message{
					ID:        fmt.Sprintf("dms-%d", time.Now().UnixNano()),
					From:      dms.UserNpub,
					To:        dms.Recipient,
					Text:      dms.MessageText,
					Timestamp: time.Now().Unix(),
				}
				s.db.SaveMessage(msg)

				if s.hub != nil {
					s.hub.RawBroadcastJSON(msg, "")
				}

				dms.Triggered = true
				s.db.SaveDeadMansSwitch(dms)
			}
		}
	}
}

