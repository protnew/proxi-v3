package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
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
// Scheduled Messages
// ---------------------------------------------------------------------------

// ScheduledMessage represents a pending message to be sent at a future time.

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
