package store

import (
	"strings"
	"database/sql"
	"fmt"
	"log"
)

// migration defines a single versioned database migration.
type migration struct {
	Version int
	Name    string
	Up      string // SQL to apply
}

// migrations is the ordered list of all migrations.
// Each migration runs exactly once. Version is tracked in schema_version table.
var migrations = []migration{
	{
		Version: 1,
		Name:    "initial_schema",
		Up: `
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    sender TEXT NOT NULL,
    recipient TEXT NOT NULL,
    text TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    encrypted INTEGER NOT NULL DEFAULT 0,
    reply_to TEXT DEFAULT '',
    forwarded_from TEXT DEFAULT '',
    attachments TEXT DEFAULT '',
    ttl INTEGER DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender);
CREATE INDEX IF NOT EXISTS idx_messages_recipient ON messages(recipient);
CREATE TABLE IF NOT EXISTS channels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    creator TEXT NOT NULL,
    subscribers INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS peers (
    id TEXT PRIMARY KEY,
    name TEXT DEFAULT '',
    public_key TEXT DEFAULT '',
    endpoint TEXT DEFAULT '',
    allowed_ips TEXT DEFAULT '0.0.0.0/0',
    last_seen INTEGER DEFAULT 0,
    connected INTEGER DEFAULT 0
);
CREATE TABLE IF NOT EXISTS profiles (
    npub TEXT PRIMARY KEY,
    display_name TEXT DEFAULT '',
    about TEXT DEFAULT '',
    picture_url TEXT DEFAULT '',
    updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS reactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id TEXT NOT NULL,
    user_npub TEXT NOT NULL,
    emoji TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE(message_id, user_npub)
);
CREATE TABLE IF NOT EXISTS read_receipts (
    message_id TEXT NOT NULL,
    user_npub TEXT NOT NULL,
    read_at INTEGER NOT NULL,
    PRIMARY KEY(message_id, user_npub)
);
CREATE TABLE IF NOT EXISTS scheduled_messages (
    id TEXT PRIMARY KEY,
    sender TEXT NOT NULL,
    recipient TEXT NOT NULL,
    text TEXT NOT NULL,
    send_at INTEGER NOT NULL,
    status TEXT DEFAULT 'pending',
    created_at INTEGER NOT NULL
);
`,
	},
	{
		Version: 2,
		Name:    "streams_and_files",
		Up: `
CREATE TABLE IF NOT EXISTS streams (
    id TEXT PRIMARY KEY,
    creator TEXT NOT NULL,
    title TEXT DEFAULT '',
    status TEXT DEFAULT 'active',
    created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS uploaded_files (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    content_type TEXT DEFAULT '',
    size INTEGER DEFAULT 0,
    data BLOB,
    uploaded_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS user_profiles (
    npub TEXT PRIMARY KEY,
    display_name TEXT DEFAULT '',
    avatar_url TEXT DEFAULT '',
    bio TEXT DEFAULT '',
    updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS file_metadata (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    size INTEGER NOT NULL,
    type TEXT DEFAULT '',
    uploaded_by TEXT DEFAULT '',
    created_at INTEGER NOT NULL
);
`,
	},
	{
		Version: 3,
		Name:    "nostr_and_identity",
		Up: `
CREATE TABLE IF NOT EXISTS nostr_events (
    id TEXT PRIMARY KEY,
    pubkey TEXT NOT NULL,
    kind INTEGER NOT NULL,
    tags TEXT DEFAULT '[]',
    content TEXT DEFAULT '',
    sig TEXT DEFAULT '',
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_nostr_kind ON nostr_events(kind);
CREATE INDEX IF NOT EXISTS idx_nostr_pubkey ON nostr_events(pubkey);
CREATE TABLE IF NOT EXISTS identity (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    npub TEXT NOT NULL DEFAULT '',
    nsec TEXT NOT NULL DEFAULT '',
    seed_phrase TEXT NOT NULL DEFAULT ''
);
`,
	},
	{
		Version: 4,
		Name:    "user_bans_reports_invites",
		Up: `
CREATE TABLE IF NOT EXISTS user_bans (
    user_id TEXT PRIMARY KEY,
    reason TEXT DEFAULT '',
    banned_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT DEFAULT ''
);
CREATE TABLE IF NOT EXISTS reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id TEXT NOT NULL DEFAULT '',
    reporter_id TEXT NOT NULL,
    reason TEXT DEFAULT '',
    status TEXT DEFAULT 'open',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    resolved_at TEXT DEFAULT '',
    resolver_id TEXT DEFAULT ''
);
CREATE TABLE IF NOT EXISTS invites (
    id TEXT PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    channel TEXT NOT NULL DEFAULT '',
    group_name TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL,
    max_uses INTEGER DEFAULT 100,
    uses INTEGER DEFAULT 0,
    expires_at INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL
);
`,
	},
	{
		Version: 5,
		Name:    "analytics_referrals",
		Up: `
CREATE TABLE IF NOT EXISTS analytics_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    event TEXT NOT NULL,
    properties TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_analytics_events_user ON analytics_events(user_id);
CREATE INDEX IF NOT EXISTS idx_analytics_events_event ON analytics_events(event);
CREATE INDEX IF NOT EXISTS idx_analytics_events_created ON analytics_events(created_at);
CREATE TABLE IF NOT EXISTS referrals (
    code TEXT PRIMARY KEY,
    referrer_id TEXT NOT NULL,
    uses INTEGER NOT NULL DEFAULT 0,
    max_uses INTEGER NOT NULL DEFAULT 100,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_id);
CREATE TABLE IF NOT EXISTS referral_uses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL,
    new_user_id TEXT NOT NULL,
    bonus_days INTEGER NOT NULL DEFAULT 30,
    used_at INTEGER NOT NULL,
    FOREIGN KEY (code) REFERENCES referrals(code)
);
`,
	},
	{
		Version: 6,
		Name:    "subscriptions_donations_stories",
		Up: `
CREATE TABLE IF NOT EXISTS subscriptions (
    user_id TEXT PRIMARY KEY,
    tier TEXT NOT NULL DEFAULT 'free',
    expires_at INTEGER,
    auto_renew INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS donations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_user TEXT NOT NULL,
    to_channel TEXT NOT NULL,
    amount_sats INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS stories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    author TEXT NOT NULL,
    media_url TEXT NOT NULL,
    media_type TEXT NOT NULL DEFAULT 'image',
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS threads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    root_id TEXT NOT NULL,
    reply_id TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS webhooks (
    id TEXT PRIMARY KEY,
    bot_id TEXT NOT NULL,
    url TEXT NOT NULL,
    secret TEXT NOT NULL DEFAULT '',
    events TEXT NOT NULL DEFAULT '[]'
);
`,
	},
	{
		Version: 7,
		Name:    "audit_brand_governance",
		Up: `
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,
    resource TEXT NOT NULL DEFAULT '',
    details TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS brand_config (
    app_name TEXT NOT NULL DEFAULT 'Proxi',
    logo_url TEXT NOT NULL DEFAULT '',
    primary_color TEXT NOT NULL DEFAULT '#6AB2F3',
    accent_color TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS votes (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL,
    initiator TEXT NOT NULL,
    question TEXT NOT NULL,
    options TEXT NOT NULL,
    ends_at INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'open'
);
CREATE TABLE IF NOT EXISTS vote_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vote_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    option TEXT NOT NULL
);
`,
	},
	{
		Version: 8,
		Name:    "users_and_prekeys",
		Up: `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    npub TEXT UNIQUE NOT NULL,
    username TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    refresh_token TEXT DEFAULT ''
);
CREATE TABLE IF NOT EXISTS prekey_bundles (
    user_id TEXT PRIMARY KEY,
    identity_key BLOB NOT NULL,
    signed_prekey BLOB NOT NULL,
    signature BLOB NOT NULL,
    one_time_prekey BLOB NOT NULL,
    created_at INTEGER NOT NULL
);
`,
	},
	{
		Version: 9,
		Name:    "fts5_search",
		Up: `
-- P1/D5: remove the destructive FTS5 probe from migration Up.
-- Destructive DDL in migration Up is forbidden (DATA ZERO TOLERANCE). No-op placeholder kept
-- so schema_version v9 stays stable; real FTS index must be created without DROP.
SELECT 1;
`,
	},
	{
		Version: 10,
		Name:    "groups_and_switch",
		Up: `
CREATE TABLE IF NOT EXISTS group_members (
	group_id TEXT NOT NULL,
	user_npub TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'member',
	joined_at INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (group_id, user_npub)
);

CREATE TABLE IF NOT EXISTS dead_mans_switch (
	id TEXT PRIMARY KEY,
	user_npub TEXT NOT NULL,
	message_text TEXT NOT NULL,
	recipient TEXT NOT NULL DEFAULT 'broadcast',
	interval_days INTEGER NOT NULL DEFAULT 7,
	last_check_in INTEGER NOT NULL DEFAULT 0,
	triggered INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL DEFAULT 0
);
`,
	},
	{
		Version: 11,
		Name:    "federation_peers",
		Up: `
CREATE TABLE IF NOT EXISTS federation_peers (
	id TEXT PRIMARY KEY,
	url TEXT UNIQUE NOT NULL,
	last_sync INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active'
);
`,
	},
	{
		Version: 12,
		Name:    "content_vault",
		Up: `
CREATE TABLE IF NOT EXISTS content_manifests (
	id TEXT PRIMARY KEY,
	owner_npub TEXT NOT NULL,
	content_type TEXT NOT NULL,
	metadata TEXT NOT NULL DEFAULT '{}',
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS content_catalog (
	id TEXT PRIMARY KEY,
	manifest_id TEXT NOT NULL,
	chunk_hash TEXT NOT NULL,
	size INTEGER NOT NULL,
	availability TEXT DEFAULT 'local',
	FOREIGN KEY (manifest_id) REFERENCES content_manifests(id)
);
`,
	},
	{
		Version: 13,
		Name:    "hls_streaming",
		Up: `
CREATE TABLE IF NOT EXISTS streams (
    id TEXT PRIMARY KEY,
    creator TEXT NOT NULL,
    title TEXT DEFAULT '',
    status TEXT DEFAULT 'active',
    created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS stream_chunks (
	id TEXT PRIMARY KEY,
	stream_id TEXT NOT NULL,
	sequence_number INTEGER NOT NULL,
	duration REAL NOT NULL,
	data BLOB,
	created_at INTEGER NOT NULL,
	FOREIGN KEY (stream_id) REFERENCES streams(id)
);
CREATE INDEX IF NOT EXISTS idx_stream_chunks_stream_id ON stream_chunks(stream_id);
CREATE INDEX IF NOT EXISTS idx_stream_chunks_sequence ON stream_chunks(stream_id, sequence_number);
`,
	},
	{
		Version: 14,
		Name:    "add_contacts_table",
		Up: `CREATE TABLE IF NOT EXISTS contacts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    public_key TEXT NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    is_messenger_friend INTEGER NOT NULL DEFAULT 0,
    grant_vpn_access INTEGER NOT NULL DEFAULT 0,
    use_as_vpn_node INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT 0
);`,
	},
	{
		Version: 15,
		Name:    "messages_soft_delete_erasure",
		Up: `
-- P1: user/admin delete → soft-delete tombstone (deleted_at).
-- SEC crypto erase → random wipe + erased_at + hard DELETE in one tx.
ALTER TABLE messages ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN deleted_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN erased_at INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_messages_is_deleted ON messages(is_deleted);
CREATE INDEX IF NOT EXISTS idx_messages_deleted_at ON messages(deleted_at);
CREATE INDEX IF NOT EXISTS idx_messages_erased_at ON messages(erased_at);
`,
	},
	{
		Version: 16,
		Name:    "identity_owner_user_id",
		Up: `
-- Singleton device identity: first JWT user to claim it owns nsec export.
ALTER TABLE identity ADD COLUMN owner_user_id TEXT NOT NULL DEFAULT '';
`,
	},
	{
		Version: 17,
		Name:    "prekey_public_only_wipe",
		Up: `
-- P2: legacy bundles stored the X25519 PRIVATE key in identity_key (server-side
-- auto-decrypt design). Wipe them; republishing now requires a verified
-- signature, so only public keys can enter the table again.
DELETE FROM prekey_bundles;
`,
	},
	{
		Version: 18,
		Name:    "contacts_owner_scoping",
		Up: `
-- P7: contacts are per-user, not a global shared list. Non-destructive
-- rebuild (no DROP): the old table is kept as contacts_legacy_v18.
ALTER TABLE contacts ADD COLUMN owner_user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE contacts RENAME TO contacts_legacy_v18;
CREATE TABLE contacts (
    id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    public_key TEXT NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    is_messenger_friend INTEGER NOT NULL DEFAULT 0,
    grant_vpn_access INTEGER NOT NULL DEFAULT 0,
    use_as_vpn_node INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT 0,
    owner_user_id TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (owner_user_id, id)
);
INSERT INTO contacts (id, name, public_key, endpoint, is_messenger_friend, grant_vpn_access, use_as_vpn_node, created_at, owner_user_id)
    SELECT id, name, public_key, endpoint, is_messenger_friend, grant_vpn_access, use_as_vpn_node, created_at, owner_user_id FROM contacts_legacy_v18;
CREATE INDEX IF NOT EXISTS idx_contacts_owner ON contacts(owner_user_id);
`,
	},
	{
		Version: 19,
		Name:    "messages_group_id",
		Up: `
-- P3: group messages keep their room id through the server (wire + REST).
ALTER TABLE messages ADD COLUMN group_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_messages_group ON messages(group_id);
`,
	},
}


// isBenignSchemaErr: column/table already present (partial legacy DBs).
func isBenignSchemaErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column name") ||
		strings.Contains(msg, "already exists")
}
// runMigrations applies all pending migrations in order.
func runMigrations(db *sql.DB) error {
	// Create schema_version tracking table
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	// Get current version
	var currentVersion int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}
		log.Printf("📦 Migration v%d: %s", m.Version, m.Name)

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration v%d: %w", m.Version, err)
		}

		if _, err := tx.Exec(m.Up); err != nil {
			if !isBenignSchemaErr(err) {
				tx.Rollback()
				return fmt.Errorf("migration v%d (%s): %w", m.Version, m.Name, err)
			}
			log.Printf("⚠️  Migration v%d (%s): benign schema skip: %v", m.Version, m.Name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_version (version, name) VALUES (?, ?)", m.Version, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration v%d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration v%d: %w", m.Version, err)
		}
		log.Printf("✅ Migration v%d applied", m.Version)
	}

	// FTS5 probe lives outside migration Up: CREATE IF NOT EXISTS, ignore errors, never DROP.
	tryEnsureFTS5(db)
	// Post-migration column repair: DBs that recorded a differently-named
	// v15 (see live messenger.db: "push_subscriptions_and_identity_owner")
	// never got the soft-delete columns. Ensure them idempotently.
	ensureColumn(db, "messages", "is_deleted", "INTEGER NOT NULL DEFAULT 0")
	ensureColumn(db, "messages", "deleted_at", "INTEGER NOT NULL DEFAULT 0")
	ensureColumn(db, "messages", "erased_at", "INTEGER NOT NULL DEFAULT 0")
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_is_deleted ON messages(is_deleted)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_deleted_at ON messages(deleted_at)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_erased_at ON messages(erased_at)`)
	return nil
}

// ensureColumn adds a column only when PRAGMA table_info shows it missing —
// safe on DBs where the same-named migration was recorded but partially applied.
func ensureColumn(db *sql.DB, table, column, decl string) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			if name == column {
				return // already present
			}
		}
	}
	if _, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + decl); err != nil {
		log.Printf("⚠️  ensureColumn %s.%s: %v", table, column, err)
	}
}

func tryEnsureFTS5(db *sql.DB) {
	// Capability probe only (separate Exec, errors ignored). Never DROP.
	// Do not create product messages_fts here: it has no content sync and SearchMessagesFTS
	// would stop falling back to LIKE.
	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS fts5_capability_probe USING fts5(x)`)
}

// GetSchemaVersion returns the current schema version.
func (s *Store) GetSchemaVersion() int {
	var v int
	s.DB().QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&v)
	return v
}
