package store

// v9–v19 feature schema. Split from migration.go (TZ-EXEC-20260924 A1).
var migrationsFeatures = []migration{
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
