// File: routing_chat.go
// P2-1 RESCUE 20260720: extracted from routing.go (God Object split).

package main

import (
	vpnroot "github.com/unkillable-messenger/vpn"
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
	// H1: scope strictly to JWT npub — ?npub= query fallback was an IDOR
	// (any token holder could pull another user's DM history).
	npub, _ := r.Context().Value("npub").(string)
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

	// Auto-decrypt messages for the current user (CRYP-010 DR first, then legacy ECDH)
	currentUserNpub := npub
	if currentUserNpub != "" {
		for i, m := range msgs {
			if !m.Encrypted || m.To != currentUserNpub {
				continue
			}
			// Double Ratchet
			if s.drSessions != nil && chat.IsDRCiphertext(m.Text) {
				pt, used, err := s.drSessions.DecryptInbound(currentUserNpub, m.From, m.Text)
				if err == nil && used {
					msgs[i].Text = pt
					msgs[i].Encrypted = false
					continue
				}
				if err != nil {
					zap.S().Warnf("DR decrypt failed msg %s from %s: %v", m.ID, m.From, err)
				}
			}
			// Legacy ECDH
			recipientBundle, err := s.db.GetPreKeyBundle(currentUserNpub)
			if err == nil && recipientBundle != nil {
				plaintext, err := chat.DecryptMessageFromSender(m.Text, recipientBundle.IdentityKey, m.From)
				if err == nil {
					msgs[i].Text = plaintext
					msgs[i].Encrypted = false
				} else {
					zap.S().Warnf("Failed to decrypt message %s from %s: %v", m.ID, m.From, err)
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
		From      string `json:"from"`
		To        string `json:"to"`
		Text      string `json:"text"`
		Encrypted bool   `json:"encrypted"`
		IsE2E     bool   `json:"is_e2e"`
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

	// P5 (2026-09-20): клиентский шифротекст хранится как есть — сервер НЕ
	// перешифровывает (иначе двойное шифрование ломает NIP-44 на приёме).
	// Fail-closed: флаг encrypted/is_e2e без формата шифротекста → 422.
	clientCipher := chat.LooksLikeClientCiphertext(req.Text)
	if (req.Encrypted || req.IsE2E) && !clientCipher {
		writeError(w, http.StatusUnprocessableEntity, "E2E_FLAG_MISMATCH", "encrypted flag without ciphertext payload")
		return
	}

	// CRYP-010: Double Ratchet first (when session exists), then legacy ECDH prekey path.
	encryptedText := req.Text
	isEncrypted := clientCipher
	if !isEncrypted && req.To != "broadcast" && req.To != "" {
		// Prefer DR session if established (forward secrecy)
		if s.drSessions != nil {
			ct, usedDR, derr := s.drSessions.EncryptOutbound(req.From, req.To, req.Text)
			if derr != nil {
				log.Printf("WARNING: DR encrypt failed: %v", derr)
			} else if usedDR {
				encryptedText = ct
				isEncrypted = true
			}
		}
		// Fallback: legacy ECDH via stored prekey bundle
		if !isEncrypted {
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

	// MSG-110: push to WebSocket hub so online recipients get real-time delivery
	// (REST save alone forced Bob to reload — hub.SendTo was never called from HTTP path)
	if s.hub != nil {
		wsMsg := &chat.Message{
			Type:  chat.TypeChat,
			ID:    msg.ID,
			From:  msg.From,
			To:    msg.To,
			Text:  msg.Text,
			Ts:    msg.Timestamp,
			IsE2E: msg.Encrypted,
		}
		if encoded, encErr := wsMsg.Encode(); encErr == nil {
			switch msg.To {
			case "", chat.BroadcastTarget:
				s.hub.Broadcast(encoded, msg.From)
				// multi-device echo to sender
				s.hub.SendTo(msg.From, encoded)
			default:
				delivered := s.hub.SendTo(msg.To, encoded)
				s.hub.SendTo(msg.From, encoded) // echo to sender devices
				if !delivered {
					log.Printf("MSG-110: recipient %s offline — message persisted only", truncate(msg.To, 16))
				}
			}
		} else {
			log.Printf("MSG-110: encode WS message failed: %v", encErr)
		}
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


// P8 (2026-09-20): startDeadMansSwitchWorker удалён — был вторым DMS-воркером
// (дубль-триггеры + broadcast всем = утечка). Остаётся deadMansSwitchLoop.

