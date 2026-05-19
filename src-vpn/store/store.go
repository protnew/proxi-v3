package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store wraps an SQLite database for messenger persistence.
type Store struct {
	db *sql.DB
}

// Message represents a stored chat message.
type Message struct {
	ID            string `json:"id"`
	From          string `json:"from"`
	To            string `json:"to"`                   // "broadcast" для публичных, npub для DM
	Text          string `json:"text"`                 // plaintext или encrypted base64
	Encrypted     bool   `json:"encrypted"`            // true = E2E encrypted
	Timestamp     int64  `json:"timestamp"`
	ReplyTo       string `json:"replyTo,omitempty"`    // ID сообщения-ответа
	ForwardedFrom string `json:"forwardedFrom,omitempty"` // npub автора пересланного
	Attachments   string `json:"attachments,omitempty"`   // JSON array of file IDs
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

// NewStore opens (or creates) an SQLite database at dbPath and runs
// auto-migration to ensure all required tables exist.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", dbPath, err)
	}

	// Enable foreign keys.
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate creates tables if they do not exist.
func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id         TEXT PRIMARY KEY,
			sender     TEXT NOT NULL,
			recipient  TEXT NOT NULL,
			text       TEXT NOT NULL,
			encrypted  INTEGER NOT NULL DEFAULT 0,
			timestamp  INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_recipient ON messages(recipient, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_timestamp   ON messages(timestamp)`,

		`CREATE TABLE IF NOT EXISTS channels (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			creator     TEXT NOT NULL,
			subscribers INTEGER NOT NULL DEFAULT 0,
			created_at  INTEGER NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS peers (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL DEFAULT '',
			public_key  TEXT NOT NULL DEFAULT '',
			endpoint    TEXT NOT NULL DEFAULT '',
			allowed_ips TEXT NOT NULL DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS identity (
			id           INTEGER PRIMARY KEY CHECK (id = 1),
			npub         TEXT NOT NULL DEFAULT '',
			nsec         TEXT NOT NULL DEFAULT '',
			seed_phrase  TEXT NOT NULL DEFAULT ''
		)`,

		// --- Telegram features tables ---

		`CREATE TABLE IF NOT EXISTS reactions (
			message_id TEXT NOT NULL,
			user_npub TEXT NOT NULL,
			emoji     TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			PRIMARY KEY (message_id, user_npub)
		)`,

		`CREATE TABLE IF NOT EXISTS read_receipts (
			message_id TEXT NOT NULL,
			user_npub  TEXT NOT NULL,
			read_at    INTEGER NOT NULL,
			PRIMARY KEY (message_id, user_npub)
		)`,

		`CREATE TABLE IF NOT EXISTS user_profiles (
			npub         TEXT PRIMARY KEY,
			display_name TEXT DEFAULT '',
			avatar_url   TEXT DEFAULT '',
			bio          TEXT DEFAULT '',
			updated_at   INTEGER NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS file_metadata (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			size       INTEGER NOT NULL,
			type       TEXT NOT NULL DEFAULT '',
			uploaded_by TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q, err)
		}
	}

	// Add new columns to messages table if they don't exist yet
	alterStmts := []string{
		`ALTER TABLE messages ADD COLUMN reply_to TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN forwarded_from TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN attachments TEXT NOT NULL DEFAULT ''`,
	}
	for _, q := range alterStmts {
		// Ignore errors — column may already exist
		s.db.Exec(q)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// SaveMessage persists a message.
func (s *Store) SaveMessage(msg Message) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO messages (id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.From, msg.To, msg.Text, boolToInt(msg.Encrypted), msg.Timestamp,
		msg.ReplyTo, msg.ForwardedFrom, msg.Attachments,
	)
	if err != nil {
		return fmt.Errorf("save message %s: %w", msg.ID, err)
	}
	return nil
}

// GetMessages returns up to `limit` messages newer than `since` (unix ts).
// It returns broadcast messages (recipient = "broadcast") plus DMs where the
// given npub is either sender or recipient.
func (s *Store) GetMessages(limit int, since int64, npub string) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments
		 FROM messages
		 WHERE timestamp > ?
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
			&m.ReplyTo, &m.ForwardedFrom, &m.Attachments); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.Encrypted = enc != 0
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
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

// ---------------------------------------------------------------------------
// Peers
// ---------------------------------------------------------------------------

// SavePeer persists a WireGuard peer configuration.
func (s *Store) SavePeer(id, name, pubKey, endpoint, allowedIPs string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO peers (id, name, public_key, endpoint, allowed_ips)
		 VALUES (?, ?, ?, ?, ?)`,
		id, name, pubKey, endpoint, allowedIPs,
	)
	if err != nil {
		return fmt.Errorf("save peer %s: %w", id, err)
	}
	return nil
}

// GetPeers returns all stored peers as a slice of maps.
func (s *Store) GetPeers() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT id, name, public_key, endpoint, allowed_ips FROM peers`,
	)
	if err != nil {
		return nil, fmt.Errorf("get peers: %w", err)
	}
	defer rows.Close()

	var peers []map[string]interface{}
	for rows.Next() {
		var id, name, pubKey, endpoint, allowedIPs string
		if err := rows.Scan(&id, &name, &pubKey, &endpoint, &allowedIPs); err != nil {
			return nil, fmt.Errorf("scan peer: %w", err)
		}
		peers = append(peers, map[string]interface{}{
			"id":          id,
			"name":        name,
			"public_key":  pubKey,
			"endpoint":    endpoint,
			"allowed_ips": allowedIPs,
		})
	}
	return peers, rows.Err()
}

// DeletePeer removes a peer by ID.
func (s *Store) DeletePeer(id string) error {
	res, err := s.db.Exec(`DELETE FROM peers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete peer %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("peer %s not found", id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Identity
// ---------------------------------------------------------------------------

// SaveIdentity persists the single identity row (npub, nsec, seed phrase).
func (s *Store) SaveIdentity(npub, nsec, seedPhrase string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO identity (id, npub, nsec, seed_phrase)
		 VALUES (1, ?, ?, ?)`,
		npub, nsec, seedPhrase,
	)
	if err != nil {
		return fmt.Errorf("save identity: %w", err)
	}
	return nil
}

// LoadIdentity reads the stored identity. It returns ("", "", "", sql.ErrNoRows)
// if no identity has been saved yet.
func (s *Store) LoadIdentity() (npub, nsec, seedPhrase string, err error) {
	err = s.db.QueryRow(
		`SELECT npub, nsec, seed_phrase FROM identity WHERE id = 1`,
	).Scan(&npub, &nsec, &seedPhrase)
	if err != nil {
		return "", "", "", fmt.Errorf("load identity: %w", err)
	}
	return npub, nsec, seedPhrase, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Reactions
// ---------------------------------------------------------------------------

// AddReaction adds or updates a reaction (emoji) from a user on a message.
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
func (s *Store) MarkRead(messageID, userNpub string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO read_receipts (message_id, user_npub, read_at)
		 VALUES (?, ?, ?)`,
		messageID, userNpub, nowUnix(),
	)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	return nil
}

// GetReadReceipts returns list of npubs who read the message.
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
func (s *Store) MarkAllRead(userNpub string, beforeTimestamp int64) error {
	// Get all message IDs for this user before the given timestamp
	rows, err := s.db.Query(
		`SELECT id FROM messages
		 WHERE timestamp <= ?
		   AND (recipient = 'broadcast' OR sender = ? OR recipient = ?)`,
		beforeTimestamp, userNpub, userNpub,
	)
	if err != nil {
		return fmt.Errorf("mark all read query: %w", err)
	}
	defer rows.Close()

	now := nowUnix()
	for rows.Next() {
		var msgID string
		if err := rows.Scan(&msgID); err != nil {
			return fmt.Errorf("scan message id: %w", err)
		}
		s.db.Exec(
			`INSERT OR REPLACE INTO read_receipts (message_id, user_npub, read_at) VALUES (?, ?, ?)`,
			msgID, userNpub, now,
		)
	}
	return rows.Err()
}

// ---------------------------------------------------------------------------
// User Profiles
// ---------------------------------------------------------------------------

// SaveProfile creates or updates a user profile.
func (s *Store) SaveProfile(npub, displayName, avatarURL, bio string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO user_profiles (npub, display_name, avatar_url, bio, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		npub, displayName, avatarURL, bio, nowUnix(),
	)
	if err != nil {
		return fmt.Errorf("save profile: %w", err)
	}
	return nil
}

// GetProfile returns a user's profile.
func (s *Store) GetProfile(npub string) (map[string]interface{}, error) {
	var displayName, avatarURL, bio string
	var updatedAt int64
	err := s.db.QueryRow(
		`SELECT display_name, avatar_url, bio, updated_at FROM user_profiles WHERE npub = ?`,
		npub,
	).Scan(&displayName, &avatarURL, &bio, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return map[string]interface{}{
		"npub":        npub,
		"displayName": displayName,
		"avatarUrl":   avatarURL,
		"bio":         bio,
		"updatedAt":   updatedAt,
	}, nil
}

// SearchProfiles searches user profiles by display name or npub.
func (s *Store) SearchProfiles(query string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT npub, display_name, avatar_url, bio, updated_at
		 FROM user_profiles
		 WHERE display_name LIKE ? OR npub LIKE ?
		 ORDER BY updated_at DESC
		 LIMIT 50`,
		"%"+query+"%", "%"+query+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("search profiles: %w", err)
	}
	defer rows.Close()

	var profiles []map[string]interface{}
	for rows.Next() {
		var npub, displayName, avatarURL, bio string
		var updatedAt int64
		if err := rows.Scan(&npub, &displayName, &avatarURL, &bio, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, map[string]interface{}{
			"npub":        npub,
			"displayName": displayName,
			"avatarUrl":   avatarURL,
			"bio":         bio,
			"updatedAt":   updatedAt,
		})
	}
	return profiles, rows.Err()
}

// ---------------------------------------------------------------------------
// Search Messages
// ---------------------------------------------------------------------------

// SearchMessages performs a text search across messages visible to the given npub.
func (s *Store) SearchMessages(query string, npub string, limit int) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments
		 FROM messages
		 WHERE text LIKE ?
		   AND (recipient = 'broadcast' OR sender = ? OR recipient = ?)
		 ORDER BY timestamp DESC
		 LIMIT ?`,
		"%"+query+"%", npub, npub, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var enc int
		if err := rows.Scan(&m.ID, &m.From, &m.To, &m.Text, &enc, &m.Timestamp,
			&m.ReplyTo, &m.ForwardedFrom, &m.Attachments); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.Encrypted = enc != 0
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// ---------------------------------------------------------------------------
// File Metadata
// ---------------------------------------------------------------------------

// FileMeta represents metadata for an uploaded file.
type FileMeta struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Type       string `json:"type"`
	UploadedBy string `json:"uploadedBy"`
	CreatedAt  int64  `json:"createdAt"`
}

// SaveFileMeta persists file metadata.
func (s *Store) SaveFileMeta(fm FileMeta) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO file_metadata (id, name, size, type, uploaded_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		fm.ID, fm.Name, fm.Size, fm.Type, fm.UploadedBy, fm.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save file meta: %w", err)
	}
	return nil
}

// GetFileMeta returns metadata for a single file.
func (s *Store) GetFileMeta(id string) (*FileMeta, error) {
	var fm FileMeta
	err := s.db.QueryRow(
		`SELECT id, name, size, type, uploaded_by, created_at FROM file_metadata WHERE id = ?`,
		id,
	).Scan(&fm.ID, &fm.Name, &fm.Size, &fm.Type, &fm.UploadedBy, &fm.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get file meta: %w", err)
	}
	return &fm, nil
}

// ListFileMeta returns metadata for all uploaded files.
func (s *Store) ListFileMeta() ([]FileMeta, error) {
	rows, err := s.db.Query(
		`SELECT id, name, size, type, uploaded_by, created_at FROM file_metadata ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list file meta: %w", err)
	}
	defer rows.Close()

	var files []FileMeta
	for rows.Next() {
		var fm FileMeta
		if err := rows.Scan(&fm.ID, &fm.Name, &fm.Size, &fm.Type, &fm.UploadedBy, &fm.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan file meta: %w", err)
		}
		files = append(files, fm)
	}
	return files, rows.Err()
}

// DeleteFileMeta removes file metadata by ID.
func (s *Store) DeleteFileMeta(id string) error {
	res, err := s.db.Exec(`DELETE FROM file_metadata WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete file meta %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("file %s not found", id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// time helper
// ---------------------------------------------------------------------------

func nowUnix() int64 {
	return time.Now().Unix()
}
