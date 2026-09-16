package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SEC-002: MUST equal vpn.MaxMessageLen (root validation.go). Keep in sync.
const MaxMessageLen = 16384 // 16 KB — SoT mirror of vpn.MaxMessageLen

type Message struct {
	ID            string `json:"id"`
	From          string `json:"from"`
	To            string `json:"to"`        // "broadcast" для публичных, npub для DM
	Text          string `json:"text"`      // plaintext или encrypted base64
	Encrypted     bool   `json:"encrypted"` // true = E2E encrypted
	Timestamp     int64  `json:"timestamp"`
	ReplyTo       string `json:"replyTo,omitempty"`       // ID сообщения-ответа
	ForwardedFrom string `json:"forwardedFrom,omitempty"` // npub автора пересланного
	Attachments   string `json:"attachments,omitempty"`   // JSON array of file IDs
	TTL           int    `json:"ttl,omitempty"`           // seconds until self-destruct (0 = never)
	Sig           string `json:"sig,omitempty"`           // CRYP-012 Ed25519 base64
}

// Channel represents a public channel.

type Channel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Creator     string `json:"creator"` // npub
	Subscribers int    `json:"subscribers"`
	CreatedAt   int64  `json:"createdAt"`
}

// Contact represents a known peer/friend with access roles.

func (s *Store) SaveMessage(msg Message) error {
	// MSG-003: never persist empty bubbles
	if len(msg.Text) == 0 || len(strings.TrimSpace(msg.Text)) == 0 {
		return nil
	}
	// SEC-002: reject oversized payloads
	if len(msg.Text) > MaxMessageLen {
		return fmt.Errorf("message too long: %d > %d", len(msg.Text), MaxMessageLen)
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO messages (id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments, ttl)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.From, msg.To, msg.Text, boolToInt(msg.Encrypted), msg.Timestamp,
		msg.ReplyTo, msg.ForwardedFrom, msg.Attachments, msg.TTL,
	)
	if err != nil {
		return fmt.Errorf("save message %s: %w", msg.ID, err)
	}
	return nil
}

// EditMessage updates the text of an existing message. Returns error if not found.

func (s *Store) EditMessage(messageID, newText, senderNpub string) error {
	res, err := s.db.Exec(
		`UPDATE messages SET text = ? WHERE id = ? AND sender = ? AND IFNULL(is_deleted, 0) = 0 AND IFNULL(deleted_at, 0) = 0`,
		newText, messageID, senderNpub,
	)
	if err != nil {
		return fmt.Errorf("edit message %s: %w", messageID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("message %s not found or unauthorized", messageID)
	}
	return nil
}

// DeleteMessage soft-deletes a user message. SEC erasure is a separate, transactional path.
// P1 assumption: ordinary user delete keeps a tombstone until an explicit retention policy purges it.

func (s *Store) DeleteMessage(messageID, senderNpub string) error {
	res, err := s.db.Exec(`UPDATE messages SET is_deleted = 1, deleted_at = ? WHERE id = ? AND sender = ? AND IFNULL(is_deleted, 0) = 0 AND IFNULL(deleted_at, 0) = 0`, time.Now().Unix(), messageID, senderNpub)
	if err != nil {
		return fmt.Errorf("delete message %s: %w", messageID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("message %s not found or unauthorized", messageID)
	}
	return nil
}

// GetMessages returns up to `limit` messages newer than `since` (unix ts).
// It returns broadcast messages (recipient = "broadcast") plus DMs where the
// given npub is either sender or recipient.

func (s *Store) GetMessages(limit int, since int64, npub string) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments, ttl
		 FROM messages
		 WHERE IFNULL(is_deleted, 0) = 0 AND IFNULL(deleted_at, 0) = 0 AND timestamp > ?
		   AND (recipient = 'broadcast'
		        OR sender = ?
		        OR recipient = ?)
		 ORDER BY timestamp ASC
		 LIMIT ?`,
		since, npub, npub, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var enc int
		if err := rows.Scan(&m.ID, &m.From, &m.To, &m.Text, &enc, &m.Timestamp,
			&m.ReplyTo, &m.ForwardedFrom, &m.Attachments, &m.TTL); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.Encrypted = enc != 0
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GetMessageByID returns a single message by ID, or error if not found.

func (s *Store) GetMessageByID(id string) (*Message, error) {
	row := s.db.QueryRow(
		`SELECT id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments, ttl
		 FROM messages WHERE id = ? AND IFNULL(is_deleted, 0) = 0 AND IFNULL(deleted_at, 0) = 0`, id,
	)
	var m Message
	var enc int
	if err := row.Scan(&m.ID, &m.From, &m.To, &m.Text, &enc, &m.Timestamp,
		&m.ReplyTo, &m.ForwardedFrom, &m.Attachments, &m.TTL); err != nil {
		return nil, fmt.Errorf("message %s not found: %w", id, err)
	}
	m.Encrypted = enc != 0
	return &m, nil
}

// ---------------------------------------------------------------------------
// Channels
// ---------------------------------------------------------------------------

// SaveChannel persists a channel (insert or update).

func (s *Store) SaveChannel(ch Channel) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO channels (id, name, description, creator, subscribers, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		ch.ID, ch.Name, ch.Description, ch.Creator, ch.Subscribers, ch.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save channel %s: %w", ch.ID, err)
	}
	return nil
}

// GetChannels returns all channels ordered by creation time.

func (s *Store) GetChannels() ([]Channel, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, creator, subscribers, created_at
		 FROM channels
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get channels: %w", err)
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Creator, &ch.Subscribers, &ch.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// SubscribeChannel increments subscriber count for a channel.

func (s *Store) SubscribeChannel(channelID string) error {
	res, err := s.db.Exec(
		`UPDATE channels SET subscribers = subscribers + 1 WHERE id = ?`,
		channelID,
	)
	if err != nil {
		return fmt.Errorf("subscribe channel %s: %w", channelID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("channel %s not found", channelID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Peers
// ---------------------------------------------------------------------------

// SaveContact creates or updates a contact.

func (s *Store) AddReaction(messageID, userNpub, emoji string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO reactions (message_id, user_npub, emoji, created_at)
		 VALUES (?, ?, ?, ?)`,
		messageID, userNpub, emoji, nowUnix(),
	)
	if err != nil {
		return fmt.Errorf("add reaction: %w", err)
	}
	return nil
}

// RemoveReaction removes a user's reaction from a message.

func (s *Store) RemoveReaction(messageID, userNpub string) error {
	_, err := s.db.Exec(
		`DELETE FROM reactions WHERE message_id = ? AND user_npub = ?`,
		messageID, userNpub,
	)
	if err != nil {
		return fmt.Errorf("remove reaction: %w", err)
	}
	return nil
}

// GetReactions returns all reactions for a message.

func (s *Store) GetReactions(messageID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT user_npub, emoji, created_at FROM reactions WHERE message_id = ?`,
		messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("get reactions: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var npub, emoji string
		var createdAt int64
		if err := rows.Scan(&npub, &emoji, &createdAt); err != nil {
			return nil, fmt.Errorf("scan reaction: %w", err)
		}
		result = append(result, map[string]interface{}{
			"userNpub":  npub,
			"emoji":     emoji,
			"createdAt": createdAt,
		})
	}
	return result, rows.Err()
}

// ---------------------------------------------------------------------------
// Read Receipts
// ---------------------------------------------------------------------------

// MarkRead marks a message as read by a user.

func (s *Store) GetReadReceipts(messageID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT user_npub FROM read_receipts WHERE message_id = ?`,
		messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("get read receipts: %w", err)
	}
	defer rows.Close()

	var npubs []string
	for rows.Next() {
		var npub string
		if err := rows.Scan(&npub); err != nil {
			return nil, fmt.Errorf("scan read receipt: %w", err)
		}
		npubs = append(npubs, npub)
	}
	return npubs, rows.Err()
}

// MarkAllRead marks all messages before a timestamp as read by a user.

func (s *Store) SearchMessages(query string, npub string, limit int) ([]Message, error) {
	var rows *sql.Rows
	var err error
	if npub == "" {
		rows, err = s.db.Query(
			`SELECT id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments, ttl
		 FROM messages
		 WHERE IFNULL(is_deleted, 0) = 0 AND IFNULL(deleted_at, 0) = 0 AND text LIKE ?
		 ORDER BY timestamp DESC
		 LIMIT ?`,
			"%"+query+"%", limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments, ttl
		 FROM messages
		 WHERE IFNULL(is_deleted, 0) = 0 AND IFNULL(deleted_at, 0) = 0 AND text LIKE ?
		   AND (recipient = 'broadcast' OR sender = ? OR recipient = ?)
		 ORDER BY timestamp DESC
		 LIMIT ?`,
			"%"+query+"%", npub, npub, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var enc int
		if err := rows.Scan(&m.ID, &m.From, &m.To, &m.Text, &enc, &m.Timestamp,
			&m.ReplyTo, &m.ForwardedFrom, &m.Attachments, &m.TTL); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.Encrypted = enc != 0
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// CleanExpiredMessages SEC-002: wipe and delete expired TTL rows atomically.
func (s *Store) CleanExpiredMessages() (n int64, err error) {
	now := time.Now().Unix()
	err = s.RunInTx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`UPDATE messages SET
				text = hex(randomblob(max(16, length(text)))),
				attachments = '',
				encrypted = 0,
				erased_at = ?
			 WHERE ttl > 0 AND (timestamp + ttl) < ? AND IFNULL(erased_at, 0) = 0`, now, now,
		); err != nil {
			return fmt.Errorf("crypto-erase expired messages: %w", err)
		}
		res, err := tx.Exec(`DELETE FROM messages WHERE erased_at IS NOT NULL AND erased_at > 0 AND ttl > 0 AND (timestamp + ttl) < ?`, now)
		if err != nil {
			return fmt.Errorf("clean expired messages: %w", err)
		}
		n, err = res.RowsAffected()
		return err
	})
	return n, err
}

// CryptoEraseMessage wipes and hard-deletes a message in one transaction (SEC-002 manual).
// This is intentionally distinct from DeleteMessage's user-visible soft delete.
func (s *Store) CryptoEraseMessage(messageID string) error {
	return s.RunInTx(context.Background(), func(tx *sql.Tx) error {
		now := time.Now().Unix()
		res, err := tx.Exec(
			`UPDATE messages SET
				text = hex(randomblob(max(16, length(text)))),
				attachments = '',
				encrypted = 0,
				erased_at = ?
			 WHERE id = ?`, now, messageID,
		)
		if err != nil {
			return fmt.Errorf("crypto-erase %s: %w", messageID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("crypto-erase rows %s: %w", messageID, err)
		}
		if n == 0 {
			return fmt.Errorf("message %s not found", messageID)
		}
		if _, err := tx.Exec(`DELETE FROM messages WHERE id = ? AND erased_at IS NOT NULL AND erased_at > 0`, messageID); err != nil {
			return fmt.Errorf("delete after erase %s: %w", messageID, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO audit_log (user_id, action, resource, details) VALUES (?, ?, ?, ?)`,
			"system", "crypto_erase", messageID, `{"tx":true,"sec":"SEC-002"}`,
		); err != nil {
			return fmt.Errorf("audit crypto-erase %s: %w", messageID, err)
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// File Metadata
// ---------------------------------------------------------------------------

// FileMeta represents metadata for an uploaded file.

type ScheduledMessage struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Text      string `json:"text"`
	SendAt    int64  `json:"sendAt"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"createdAt"`
}

// SaveScheduledMessage persists a scheduled message.

func (s *Store) SaveScheduledMessage(msg ScheduledMessage) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO scheduled_messages (id, sender, recipient, text, send_at, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.Sender, msg.Recipient, msg.Text, msg.SendAt, msg.Status, msg.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save scheduled message %s: %w", msg.ID, err)
	}
	return nil
}

// GetPendingScheduled returns all pending messages where send_at <= now.

func (s *Store) GetPendingScheduled() ([]ScheduledMessage, error) {
	now := time.Now().Unix()
	rows, err := s.db.Query(
		`SELECT id, sender, recipient, text, send_at, status, created_at
		 FROM scheduled_messages WHERE status = 'pending' AND send_at <= ?`, now,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending scheduled: %w", err)
	}
	defer rows.Close()

	var msgs []ScheduledMessage
	for rows.Next() {
		var m ScheduledMessage
		if err := rows.Scan(&m.ID, &m.Sender, &m.Recipient, &m.Text, &m.SendAt, &m.Status, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan scheduled: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// MarkScheduledSent updates a scheduled message status to 'sent'.

func (s *Store) MarkScheduledSent(id string) error {
	_, err := s.db.Exec(`UPDATE scheduled_messages SET status = 'sent' WHERE id = ?`, id)
	return err
}

// ---------------------------------------------------------------------------
// Dead Man's Switch
// ---------------------------------------------------------------------------

// DeadMansSwitch represents a message that will be sent if the user doesn't check in.

// DeleteEmptyMessages removes leftover SEC-erasure rows (erased_at set AND TTL expired).
// Empty/whitespace text alone is never a deletion predicate (P1 data-loss gate).
// allowEmptyCleanup default callers MUST pass false; true only purges marked leftovers.
func (s *Store) DeleteEmptyMessages(allowEmptyCleanup bool) (int64, error) {
	if !allowEmptyCleanup {
		return 0, nil
	}
	now := time.Now().Unix()
	res, err := s.db.Exec(
		`DELETE FROM messages
		 WHERE erased_at IS NOT NULL AND erased_at > 0
		   AND ttl > 0 AND (timestamp + ttl) < ?`, now,
	)
	if err != nil {
		return 0, fmt.Errorf("delete empty messages: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
