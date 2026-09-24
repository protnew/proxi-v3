package store

// v1–v8 core schema. Split from migration.go (TZ-EXEC-20260924 A1).
var migrationsCore = []migration{
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
}
