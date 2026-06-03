package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store wraps an SQLite database for messenger persistence.
type Store struct {
	db *sql.DB
}

// DB returns the underlying database connection (for admin operations).
func (s *Store) DB() *sql.DB {
	return s.db
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
	TTL           int    `json:"ttl,omitempty"`            // seconds until self-destruct (0 = never)
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
	if err := runMigrations(db); err != nil {
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
		`ALTER TABLE messages ADD COLUMN ttl INTEGER NOT NULL DEFAULT 0`,
	}
	for _, q := range alterStmts {
		// Ignore errors — column may already exist
		s.db.Exec(q)
	}

	// Scheduled messages table
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS scheduled_messages (
			id TEXT PRIMARY KEY,
			sender TEXT NOT NULL,
			recipient TEXT NOT NULL DEFAULT 'broadcast',
			text TEXT NOT NULL,
			send_at INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		return fmt.Errorf("create scheduled_messages: %w", err)
	}

	// Dead man's switch table
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS dead_mans_switch (
			id TEXT PRIMARY KEY,
			user_npub TEXT NOT NULL,
			message_text TEXT NOT NULL,
			recipient TEXT NOT NULL DEFAULT 'broadcast',
			interval_days INTEGER NOT NULL DEFAULT 7,
			last_check_in INTEGER NOT NULL DEFAULT 0,
			triggered INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		return fmt.Errorf("create dead_mans_switch: %w", err)
	}

	// Group members table
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS group_members (
			group_id TEXT NOT NULL,
			user_npub TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			joined_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (group_id, user_npub)
		)`); err != nil {
		return fmt.Errorf("create group_members: %w", err)
	}

	// Nostr events table (NIP-01 relay persistence)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS nostr_events (
			id         TEXT PRIMARY KEY,
			pubkey     TEXT NOT NULL,
			kind       INTEGER NOT NULL,
			tags       TEXT NOT NULL DEFAULT '[]',
			content    TEXT NOT NULL DEFAULT '',
			sig        TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create nostr_events: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_nostr_events_kind ON nostr_events(kind)`); err != nil {
		return fmt.Errorf("create index nostr_events_kind: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_nostr_events_pubkey ON nostr_events(pubkey)`); err != nil {
		return fmt.Errorf("create index nostr_events_pubkey: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_nostr_events_created_at ON nostr_events(created_at)`); err != nil {
		return fmt.Errorf("create index nostr_events_created_at: %w", err)
	}

	// Push subscriptions table (Web Push VAPID)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS push_subscriptions (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			endpoint   TEXT NOT NULL,
			p256dh     TEXT NOT NULL DEFAULT '',
			auth       TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT 0,
			UNIQUE(user_id, endpoint)
		)`); err != nil {
		return fmt.Errorf("create push_subscriptions: %w", err)
	}

	// Sticker packs table
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sticker_packs (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			owner_npub TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create sticker_packs: %w", err)
	}

	// Stickers table
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS stickers (
			id      TEXT PRIMARY KEY,
			pack_id TEXT NOT NULL,
			url     TEXT NOT NULL DEFAULT '',
			emoji   TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (pack_id) REFERENCES sticker_packs(id)
		)`); err != nil {
		return fmt.Errorf("create stickers: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_stickers_pack ON stickers(pack_id)`); err != nil {
		return fmt.Errorf("create index stickers_pack: %w", err)
	}

	// Bots table
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS bots (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			token      TEXT NOT NULL DEFAULT '',
			owner_npub TEXT NOT NULL DEFAULT '',
			command    TEXT NOT NULL DEFAULT '',
			active     INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create bots: %w", err)
	}

	// Streams table
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS streams (
			id           TEXT PRIMARY KEY,
			channel_name TEXT NOT NULL DEFAULT '',
			streamer_id  TEXT NOT NULL DEFAULT '',
			viewer_count INTEGER NOT NULL DEFAULT 0,
			active       INTEGER NOT NULL DEFAULT 1,
			started_at   INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create streams: %w", err)
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_bans (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id   TEXT NOT NULL,
			reason    TEXT,
			banned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME
		)`); err != nil {
		return fmt.Errorf("create user_bans: %w", err)
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS reports (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id  INTEGER,
			reporter_id TEXT,
			reason      TEXT,
			status      TEXT DEFAULT 'open',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME,
			resolver_id TEXT
		)`); err != nil {
		return fmt.Errorf("create reports: %w", err)
	}

	// Federation peers table (Sprint 6 — S6.1)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS federation_peers (
			id        TEXT PRIMARY KEY,
			url       TEXT NOT NULL UNIQUE,
			last_sync INTEGER NOT NULL DEFAULT 0,
			status    TEXT NOT NULL DEFAULT 'active'
		)`); err != nil {
		return fmt.Errorf("create federation_peers: %w", err)
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS invites (
			id          TEXT PRIMARY KEY,
			channel     TEXT DEFAULT '',
			group_name  TEXT DEFAULT '',
			created_by  TEXT NOT NULL,
			expires_at  INTEGER DEFAULT 0,
			max_uses    INTEGER DEFAULT 0,
			uses        INTEGER DEFAULT 0
		)`); err != nil {
		return fmt.Errorf("create invites: %w", err)
	}

	// Analytics events table (Sprint 9 — S9.2)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS analytics_events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			event      TEXT NOT NULL,
			properties TEXT NOT NULL DEFAULT '{}',
			created_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create analytics_events: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_analytics_events_user ON analytics_events(user_id)`); err != nil {
		return fmt.Errorf("create index analytics_events_user: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_analytics_events_event ON analytics_events(event)`); err != nil {
		return fmt.Errorf("create index analytics_events_event: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_analytics_events_created ON analytics_events(created_at)`); err != nil {
		return fmt.Errorf("create index analytics_events_created: %w", err)
	}

	// Referrals table (Sprint 9 — S9.3)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS referrals (
			code        TEXT PRIMARY KEY,
			referrer_id TEXT NOT NULL,
			uses        INTEGER NOT NULL DEFAULT 0,
			max_uses    INTEGER NOT NULL DEFAULT 100,
			created_at  INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create referrals: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_id)`); err != nil {
		return fmt.Errorf("create index referrals_referrer: %w", err)
	}

	// Referral uses tracking table
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS referral_uses (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			code        TEXT NOT NULL,
			new_user_id TEXT NOT NULL,
			bonus_days  INTEGER NOT NULL DEFAULT 30,
			used_at     INTEGER NOT NULL,
			FOREIGN KEY (code) REFERENCES referrals(code)
		)`); err != nil {
		return fmt.Errorf("create referral_uses: %w", err)
	}

	// Subscriptions table (Sprint 7 — Premium)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS subscriptions (
			user_id     TEXT PRIMARY KEY,
			tier        TEXT NOT NULL DEFAULT 'free',
			expires_at  INTEGER,
			auto_renew  INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		return fmt.Errorf("create subscriptions: %w", err)
	}

	// Donations table (Sprint 7 — Lightning)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS donations (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			from_user   TEXT NOT NULL,
			to_channel  TEXT NOT NULL,
			amount_sats INTEGER NOT NULL,
			created_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`); err != nil {
		return fmt.Errorf("create donations: %w", err)
	}

	// Stories table (Sprint 8 — disappearing content)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS stories (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			author      TEXT NOT NULL,
			media_url   TEXT NOT NULL,
			media_type  TEXT NOT NULL DEFAULT 'image',
			created_at  INTEGER NOT NULL,
			expires_at  INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create stories: %w", err)
	}

	// Threads table (Sprint 8 — threaded replies)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS threads (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			root_id  TEXT NOT NULL,
			reply_id TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create threads: %w", err)
	}

	// Webhooks table (Sprint 8 — bot webhooks)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS webhooks (
			id     TEXT PRIMARY KEY,
			bot_id TEXT NOT NULL,
			url    TEXT NOT NULL,
			secret TEXT NOT NULL DEFAULT '',
			events TEXT NOT NULL DEFAULT '[]'
		)`); err != nil {
		return fmt.Errorf("create webhooks: %w", err)
	}

	// Audit log table (Sprint 11 — compliance)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_log (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			action     TEXT NOT NULL,
			resource   TEXT NOT NULL DEFAULT '',
			details    TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`); err != nil {
		return fmt.Errorf("create audit_log: %w", err)
	}

	// Brand config table (Sprint 11 — white-label)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_config (
			app_name      TEXT NOT NULL DEFAULT 'Proxi',
			logo_url      TEXT NOT NULL DEFAULT '',
			primary_color TEXT NOT NULL DEFAULT '#6AB2F3',
			accent_color  TEXT NOT NULL DEFAULT ''
		)`); err != nil {
		return fmt.Errorf("create brand_config: %w", err)
	}

	// Votes table (Sprint 12 — governance)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS votes (
			id         TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			initiator  TEXT NOT NULL,
			question   TEXT NOT NULL,
			options    TEXT NOT NULL,
			ends_at    INTEGER NOT NULL,
			status     TEXT NOT NULL DEFAULT 'open'
		)`); err != nil {
		return fmt.Errorf("create votes: %w", err)
	}

	// Vote records table (Sprint 12 — governance)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS vote_records (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			vote_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			option  TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create vote_records: %w", err)
	}

	// Users table (JWT auth)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			npub          TEXT UNIQUE,
			username      TEXT NOT NULL DEFAULT '',
			password_hash TEXT NOT NULL DEFAULT '',
			created_at    INTEGER NOT NULL,
			refresh_token TEXT NOT NULL DEFAULT ''
		)`); err != nil {
		return fmt.Errorf("create users: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_npub ON users(npub)`); err != nil {
		return fmt.Errorf("create index users_npub: %w", err)
	}

	// PreKey bundles table (E2E key exchange)
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS prekey_bundles (
			user_id         TEXT PRIMARY KEY,
			identity_key    BLOB NOT NULL,
			signed_prekey   BLOB NOT NULL,
			signature       BLOB NOT NULL,
			one_time_prekey BLOB NOT NULL,
			created_at      INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create prekey_bundles: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// SaveMessage persists a message.
func (s *Store) SaveMessage(msg Message) error {
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
func (s *Store) EditMessage(messageID, newText string) error {
	res, err := s.db.Exec(
		`UPDATE messages SET text = ? WHERE id = ?`,
		newText, messageID,
	)
	if err != nil {
		return fmt.Errorf("edit message %s: %w", messageID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("message %s not found", messageID)
	}
	return nil
}

// DeleteMessage removes a message by ID. Returns error if not found.
func (s *Store) DeleteMessage(messageID string) error {
	res, err := s.db.Exec(`DELETE FROM messages WHERE id = ?`, messageID)
	if err != nil {
		return fmt.Errorf("delete message %s: %w", messageID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("message %s not found", messageID)
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
		 FROM messages WHERE id = ?`, id,
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

	var msgIDs []string
	for rows.Next() {
		var msgID string
		if err := rows.Scan(&msgID); err != nil {
			rows.Close()
			return fmt.Errorf("scan message id: %w", err)
		}
		msgIDs = append(msgIDs, msgID)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}

	now := nowUnix()
	for _, msgID := range msgIDs {
		if _, err := s.db.Exec(
			`INSERT OR REPLACE INTO read_receipts (message_id, user_npub, read_at) VALUES (?, ?, ?)`,
			msgID, userNpub, now,
		); err != nil {
			return fmt.Errorf("mark read %s: %w", msgID, err)
		}
	}
	return nil
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
	var rows *sql.Rows
	var err error
	if npub == "" {
		rows, err = s.db.Query(
			`SELECT id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments, ttl
		 FROM messages
		 WHERE text LIKE ?
		 ORDER BY timestamp DESC
		 LIMIT ?`,
			"%"+query+"%", limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments, ttl
		 FROM messages
		 WHERE text LIKE ?
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

// CleanExpiredMessages deletes all messages where TTL > 0 and timestamp+ttl < now.
// Returns the number of deleted messages.
func (s *Store) CleanExpiredMessages() (int64, error) {
	now := time.Now().Unix()
	res, err := s.db.Exec(
		`DELETE FROM messages WHERE ttl > 0 AND (timestamp + ttl) < ?`, now,
	)
	if err != nil {
		return 0, fmt.Errorf("clean expired messages: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
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

// ---------------------------------------------------------------------------
// Scheduled Messages
// ---------------------------------------------------------------------------

// ScheduledMessage represents a pending message to be sent at a future time.
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
type DeadMansSwitch struct {
	ID           string `json:"id"`
	UserNpub     string `json:"userNpub"`
	MessageText  string `json:"messageText"`
	Recipient    string `json:"recipient"`
	IntervalDays int    `json:"intervalDays"`
	LastCheckIn  int64  `json:"lastCheckIn"`
	Triggered    bool   `json:"triggered"`
	CreatedAt    int64  `json:"createdAt"`
}

// SaveDeadMansSwitch persists a dead man's switch configuration.
func (s *Store) SaveDeadMansSwitch(dms DeadMansSwitch) error {
	triggered := 0
	if dms.Triggered {
		triggered = 1
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO dead_mans_switch (id, user_npub, message_text, recipient, interval_days, last_check_in, triggered, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		dms.ID, dms.UserNpub, dms.MessageText, dms.Recipient, dms.IntervalDays, dms.LastCheckIn, triggered, dms.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save dead mans switch %s: %w", dms.ID, err)
	}
	return nil
}

// CheckInDeadMansSwitch updates the last_check_in timestamp for all switches of a user.
func (s *Store) CheckInDeadMansSwitch(userNpub string) error {
	_, err := s.db.Exec(
		`UPDATE dead_mans_switch SET last_check_in = ? WHERE user_npub = ?`,
		time.Now().Unix(), userNpub,
	)
	return err
}

// GetExpiredSwitches returns all switches that have exceeded their interval.
func (s *Store) GetExpiredSwitches() ([]DeadMansSwitch, error) {
	now := time.Now().Unix()
	rows, err := s.db.Query(
		`SELECT id, user_npub, message_text, recipient, interval_days, last_check_in, triggered, created_at
		 FROM dead_mans_switch
		 WHERE triggered = 0 AND (last_check_in + interval_days * 86400) < ?`, now,
	)
	if err != nil {
		return nil, fmt.Errorf("get expired switches: %w", err)
	}
	defer rows.Close()

	var switches []DeadMansSwitch
	for rows.Next() {
		var dms DeadMansSwitch
		var triggered int
		if err := rows.Scan(&dms.ID, &dms.UserNpub, &dms.MessageText, &dms.Recipient,
			&dms.IntervalDays, &dms.LastCheckIn, &triggered, &dms.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan switch: %w", err)
		}
		dms.Triggered = triggered != 0
		switches = append(switches, dms)
	}
	return switches, rows.Err()
}

// MarkSwitchTriggered marks a switch as triggered.
func (s *Store) MarkSwitchTriggered(id string) error {
	_, err := s.db.Exec(`UPDATE dead_mans_switch SET triggered = 1 WHERE id = ?`, id)
	return err
}

// ---------------------------------------------------------------------------
// Group Members
// ---------------------------------------------------------------------------

// GroupMember represents a user in a group chat.
type GroupMember struct {
	GroupID  string `json:"groupId"`
	UserNpub string `json:"userNpub"`
	Role     string `json:"role"` // admin, moderator, member
	JoinedAt int64  `json:"joinedAt"`
}

// SaveGroupMember adds a user to a group.
func (s *Store) SaveGroupMember(gm GroupMember) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO group_members (group_id, user_npub, role, joined_at) VALUES (?, ?, ?, ?)`,
		gm.GroupID, gm.UserNpub, gm.Role, gm.JoinedAt,
	)
	if err != nil {
		return fmt.Errorf("save group member %s/%s: %w", gm.GroupID, gm.UserNpub, err)
	}
	return nil
}

// GetGroupMembers returns all members of a group.
func (s *Store) GetGroupMembers(groupID string) ([]GroupMember, error) {
	rows, err := s.db.Query(
		`SELECT group_id, user_npub, role, joined_at FROM group_members WHERE group_id = ?`,
		groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("get group members %s: %w", groupID, err)
	}
	defer rows.Close()

	var members []GroupMember
	for rows.Next() {
		var m GroupMember
		if err := rows.Scan(&m.GroupID, &m.UserNpub, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// RemoveGroupMember removes a user from a group.
func (s *Store) RemoveGroupMember(groupID, userNpub string) error {
	res, err := s.db.Exec(`DELETE FROM group_members WHERE group_id = ? AND user_npub = ?`, groupID, userNpub)
	if err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("member %s not found in group %s", userNpub, groupID)
	}
	return nil
}

// IsGroupAdmin checks if a user is admin of a group.
func (s *Store) IsGroupAdmin(groupID, userNpub string) (bool, error) {
	var role string
	err := s.db.QueryRow(
		`SELECT role FROM group_members WHERE group_id = ? AND user_npub = ?`,
		groupID, userNpub,
	).Scan(&role)
	if err != nil {
		return false, err
	}
	return role == "admin", nil
}

// UpdateGroupMemberRole changes a member's role.
func (s *Store) UpdateGroupMemberRole(groupID, userNpub, newRole string) error {
	_, err := s.db.Exec(
		`UPDATE group_members SET role = ? WHERE group_id = ? AND user_npub = ?`,
		newRole, groupID, userNpub,
	)
	return err
}

// ---------------------------------------------------------------------------
// Nostr Events
// ---------------------------------------------------------------------------

// NostrEvent represents a stored NIP-01 event for the relay.
type NostrEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
	CreatedAt int64      `json:"created_at"`
}

// SaveNostrEvent persists a Nostr event. Uses INSERT OR IGNORE to skip duplicates.
func (s *Store) SaveNostrEvent(evt NostrEvent) error {
	tagsJSON, err := json.Marshal(evt.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO nostr_events (id, pubkey, kind, tags, content, sig, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		evt.ID, evt.PubKey, evt.Kind, string(tagsJSON), evt.Content, evt.Sig, evt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save nostr event %s: %w", evt.ID, err)
	}
	return nil
}

// NostrEventFilter holds filter parameters for querying events.
type NostrEventFilter struct {
	Kinds   []int
	Authors []string
	Since   *int64
	Until   *int64
	Limit   int
	IDs     []string
}

// GetNostrEvents retrieves events matching the given filter.
// Results are ordered by created_at DESC (newest first).
func (s *Store) GetNostrEvents(filter NostrEventFilter) ([]NostrEvent, error) {
	var conditions []string
	var args []interface{}

	if len(filter.IDs) > 0 {
		placeholders := make([]string, len(filter.IDs))
		for i, id := range filter.IDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "id IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(filter.Authors) > 0 {
		placeholders := make([]string, len(filter.Authors))
		for i, a := range filter.Authors {
			placeholders[i] = "?"
			args = append(args, a)
		}
		conditions = append(conditions, "pubkey IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(filter.Kinds) > 0 {
		placeholders := make([]string, len(filter.Kinds))
		for i, k := range filter.Kinds {
			placeholders[i] = "?"
			args = append(args, k)
		}
		conditions = append(conditions, "kind IN ("+strings.Join(placeholders, ",")+")")
	}

	if filter.Since != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, *filter.Since)
	}

	if filter.Until != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, *filter.Until)
	}

	query := "SELECT id, pubkey, kind, tags, content, sig, created_at FROM nostr_events"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get nostr events: %w", err)
	}
	defer rows.Close()

	var events []NostrEvent
	for rows.Next() {
		var evt NostrEvent
		var tagsJSON string
		if err := rows.Scan(&evt.ID, &evt.PubKey, &evt.Kind, &tagsJSON, &evt.Content, &evt.Sig, &evt.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan nostr event: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &evt.Tags); err != nil {
			// If tags are malformed, skip them
			evt.Tags = nil
		}
		events = append(events, evt)
	}
	return events, rows.Err()
}

// DeleteOldNostrEvents removes events older than the given unix timestamp.
// Returns the number of deleted events.
func (s *Store) DeleteOldNostrEvents(olderThan int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM nostr_events WHERE created_at < ?`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("delete old nostr events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ---------------------------------------------------------------------------
// Push Subscriptions (Web Push VAPID)
// ---------------------------------------------------------------------------

// PushSubscription represents a Web Push subscription stored in SQLite.
type PushSubscription struct {
	ID        int64  `json:"id"`
	UserID    string `json:"userId"`
	Endpoint  string `json:"endpoint"`
	P256DH    string `json:"p256dh"` // subscriber's ECDH P-256 public key (base64url)
	Auth      string `json:"auth"`   // subscriber's auth secret (16 bytes, base64url)
	CreatedAt int64  `json:"createdAt"`
}

// SavePushSubscription saves or updates a Web Push subscription for a user.
func (s *Store) SavePushSubscription(userID, endpoint, p256dh, auth string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO push_subscriptions (user_id, endpoint, p256dh, auth, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		userID, endpoint, p256dh, auth, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("save push subscription: %w", err)
	}
	return nil
}

// GetPushSubscriptions retrieves push subscriptions for the given user IDs.
func (s *Store) GetPushSubscriptions(userIDs []string) ([]PushSubscription, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `SELECT id, user_id, endpoint, p256dh, auth, created_at
			  FROM push_subscriptions
			  WHERE user_id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get push subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []PushSubscription
	for rows.Next() {
		var sub PushSubscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256DH, &sub.Auth, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan push subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// ---------------------------------------------------------------------------
// Federation Peers (Sprint 6 — S6.1)
// ---------------------------------------------------------------------------

// FederationPeer represents a persisted federation peer entry.
type FederationPeer struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	LastSync  int64  `json:"lastSync"`
	Status    string `json:"status"`
}

// SaveFederationPeer inserts or updates a federation peer.
func (s *Store) SaveFederationPeer(peer FederationPeer) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO federation_peers (id, url, last_sync, status) VALUES (?, ?, ?, ?)`,
		peer.ID, peer.URL, peer.LastSync, peer.Status,
	)
	if err != nil {
		return fmt.Errorf("save federation peer %s: %w", peer.URL, err)
	}
	return nil
}

// GetFederationPeers returns all federation peers.
func (s *Store) GetFederationPeers() ([]FederationPeer, error) {
	rows, err := s.db.Query(`SELECT id, url, last_sync, status FROM federation_peers`)
	if err != nil {
		return nil, fmt.Errorf("get federation peers: %w", err)
	}
	defer rows.Close()

	var peers []FederationPeer
	for rows.Next() {
		var p FederationPeer
		if err := rows.Scan(&p.ID, &p.URL, &p.LastSync, &p.Status); err != nil {
			return nil, fmt.Errorf("scan federation peer: %w", err)
		}
		peers = append(peers, p)
	}
	return peers, rows.Err()
}

// DeleteFederationPeer removes a federation peer by URL.
func (s *Store) DeleteFederationPeer(url string) error {
	res, err := s.db.Exec(`DELETE FROM federation_peers WHERE url = ?`, url)
	if err != nil {
		return fmt.Errorf("delete federation peer %s: %w", url, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("federation peer %s not found", url)
	}
	return nil
}

// UpdateFederationPeerSync updates the last_sync timestamp for a peer.
func (s *Store) UpdateFederationPeerSync(url string, lastSync int64) error {
	_, err := s.db.Exec(`UPDATE federation_peers SET last_sync = ?, status = 'active' WHERE url = ?`, lastSync, url)
	if err != nil {
		return fmt.Errorf("update federation peer sync %s: %w", url, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Users (JWT auth)
// ---------------------------------------------------------------------------

// User represents a registered user account.
type User struct {
	ID           string `json:"id"`
	Npub         string `json:"npub"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	CreatedAt    int64  `json:"createdAt"`
	RefreshToken string `json:"-"`
}

// SaveUser creates or updates a user record.
func (s *Store) SaveUser(user User) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO users (id, npub, username, password_hash, created_at, refresh_token)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		user.ID, user.Npub, user.Username, user.PasswordHash, user.CreatedAt, user.RefreshToken,
	)
	if err != nil {
		return fmt.Errorf("save user %s: %w", user.ID, err)
	}
	return nil
}

// GetUserByNpub returns a user by their npub (Nostr public key).
func (s *Store) GetUserByNpub(npub string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, npub, username, password_hash, created_at, refresh_token FROM users WHERE npub = ?`,
		npub,
	).Scan(&u.ID, &u.Npub, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("get user by npub %s: %w", npub, err)
	}
	return &u, nil
}

// GetUserByID returns a user by their ID.
func (s *Store) GetUserByID(id string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, npub, username, password_hash, created_at, refresh_token FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Npub, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("get user by id %s: %w", id, err)
	}
	return &u, nil
}

// UpdateRefreshToken updates the refresh token for a user.
func (s *Store) UpdateRefreshToken(id, token string) error {
	_, err := s.db.Exec(`UPDATE users SET refresh_token = ? WHERE id = ?`, token, id)
	if err != nil {
		return fmt.Errorf("update refresh token for %s: %w", id, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// PreKey Bundles (E2E key exchange)
// ---------------------------------------------------------------------------

// PreKeyBundleRow represents a stored prekey bundle.
type PreKeyBundleRow struct {
	UserID        string `json:"userId"`
	IdentityKey   []byte `json:"identityKey"`
	SignedPreKey  []byte `json:"signedPreKey"`
	Signature     []byte `json:"signature"`
	OneTimePreKey []byte `json:"oneTimePreKey"`
	CreatedAt     int64  `json:"createdAt"`
}

// StorePreKeyBundle persists a prekey bundle for a user.
func (s *Store) StorePreKeyBundle(userID string, identityKey, signedPreKey, signature, oneTimePreKey []byte) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO prekey_bundles (user_id, identity_key, signed_prekey, signature, one_time_prekey, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		userID, identityKey, signedPreKey, signature, oneTimePreKey, nowUnix(),
	)
	if err != nil {
		return fmt.Errorf("store prekey bundle for %s: %w", userID, err)
	}
	return nil
}

// GetPreKeyBundle retrieves a prekey bundle for a user.
func (s *Store) GetPreKeyBundle(userID string) (*PreKeyBundleRow, error) {
	var p PreKeyBundleRow
	err := s.db.QueryRow(
		`SELECT user_id, identity_key, signed_prekey, signature, one_time_prekey, created_at
		 FROM prekey_bundles WHERE user_id = ?`,
		userID,
	).Scan(&p.UserID, &p.IdentityKey, &p.SignedPreKey, &p.Signature, &p.OneTimePreKey, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get prekey bundle for %s: %w", userID, err)
	}
	return &p, nil
}
