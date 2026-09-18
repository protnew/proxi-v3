package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db          *sql.DB
	identityKey [32]byte // P6: AES-256 key outside SQLite for nsec/seed_phrase
	keyPath     string   // where the key came from (file path / env / :memory:)
}

// DB returns the underlying database connection (for admin operations).

func (s *Store) DB() *sql.DB {
	return s.db
}

// Message represents a stored chat message.

type Contact struct {
	ID                string `json:"id"` // npub
	Name              string `json:"name"`
	PublicKey         string `json:"publicKey"`
	Endpoint          string `json:"endpoint"`
	IsMessengerFriend bool   `json:"isMessengerFriend"`
	GrantVPNAccess    bool   `json:"grantVpnAccess"`
	UseAsVPNNode      bool   `json:"useAsVpnNode"`
	CreatedAt         int64  `json:"createdAt"`
}

// NewStore opens (or creates) an SQLite database at dbPath and runs
// auto-migration to ensure all required tables exist.

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", dbPath, err)
	}

	// Enable foreign keys.
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Set synchronous = NORMAL for better WAL performance.
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set synchronous=NORMAL: %w", err)
	}

	key, keyPath, err := loadOrCreateIdentityKey(dbPath)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("identity key: %w", err)
	}

	s := &Store{db: db, identityKey: key, keyPath: keyPath}
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := s.migrateIdentityAtRest(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate identity at rest: %w", err)
	}
	return s, nil
}

// Close releases the database handle.

func (s *Store) Close() error {
	return s.db.Close()
}

// migrate creates tables if they do not exist.

func (s *Store) SaveContact(c Contact) error {
	_, err := s.db.Exec(
		`INSERT INTO contacts (id, name, public_key, endpoint, is_messenger_friend, grant_vpn_access, use_as_vpn_node, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			public_key=excluded.public_key,
			endpoint=excluded.endpoint,
			is_messenger_friend=excluded.is_messenger_friend,
			grant_vpn_access=excluded.grant_vpn_access,
			use_as_vpn_node=excluded.use_as_vpn_node`,
		c.ID, c.Name, c.PublicKey, c.Endpoint,
		c.IsMessengerFriend, c.GrantVPNAccess, c.UseAsVPNNode,
		time.Now().Unix(),
	)
	return err
}

// GetContacts retrieves all stored contacts.

func (s *Store) GetContacts() ([]Contact, error) {
	rows, err := s.db.Query(`SELECT id, name, public_key, endpoint, is_messenger_friend, grant_vpn_access, use_as_vpn_node, created_at FROM contacts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Name, &c.PublicKey, &c.Endpoint, &c.IsMessengerFriend, &c.GrantVPNAccess, &c.UseAsVPNNode, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteContact removes a contact by ID.

func (s *Store) DeleteContact(id string) error {
	_, err := s.db.Exec(`DELETE FROM contacts WHERE id = ?`, id)
	return err
}

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

func nowUnix() int64 {
	return time.Now().Unix()
}

// ---------------------------------------------------------------------------
// Scheduled Messages (moved to store_deadman.go)
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// Group Members & Nostr Events (moved to store_nostr.go)
// ---------------------------------------------------------------------------


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

	query := `SELECT rowid, user_id, endpoint, p256dh, auth, created_at
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
	ID       string `json:"id"`
	URL      string `json:"url"`
	LastSync int64  `json:"lastSync"`
	Status   string `json:"status"`
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

func (s *Store) UpdateRefreshToken(id, token string) error {
	_, err := s.db.Exec(`UPDATE users SET refresh_token = ? WHERE id = ?`, token, id)
	if err != nil {
		return fmt.Errorf("update refresh token for %s: %w", id, err)
	}
	return nil
}
