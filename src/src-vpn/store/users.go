package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

func (s *Store) SaveIdentity(npub, nsec, seedPhrase string) error {
	// P6: encrypt nsec + seed_phrase at rest; npub stays plaintext (public).
	sealedNsec, err := s.sealField(nsec)
	if err != nil {
		return err
	}
	sealedSeed, err := s.sealField(seedPhrase)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO identity (id, npub, nsec, seed_phrase)
		 VALUES (1, ?, ?, ?)`,
		npub, sealedNsec, sealedSeed,
	)
	if err != nil {
		return fmt.Errorf("save identity: %w", err)
	}
	return nil
}

// LoadIdentityOwner returns the JWT user who claimed the singleton identity row.
func (s *Store) LoadIdentityOwner() string {
	var owner string
	err := s.db.QueryRow(`SELECT COALESCE(owner_user_id, '') FROM identity WHERE id = 1`).Scan(&owner)
	if err != nil {
		return ""
	}
	return owner
}

// SetIdentityOwner records the first authenticated caller as identity owner.
func (s *Store) SetIdentityOwner(userID string) error {
	if userID == "" {
		return nil
	}
	_, err := s.db.Exec(
		`UPDATE identity SET owner_user_id = ? WHERE id = 1 AND (owner_user_id = '' OR owner_user_id IS NULL)`,
		userID,
	)
	return err
}

// LoadIdentity reads the stored identity. It returns ("", "", "", sql.ErrNoRows)
// if no identity has been saved yet.

func (s *Store) LoadIdentity() (npub, nsec, seedPhrase string, err error) {
	var sealedNsec, sealedSeed string
	err = s.db.QueryRow(
		`SELECT npub, nsec, seed_phrase FROM identity WHERE id = 1`,
	).Scan(&npub, &sealedNsec, &sealedSeed)
	if err != nil {
		return "", "", "", fmt.Errorf("load identity: %w", err)
	}
	nsec, err = s.openField(sealedNsec)
	if err != nil {
		return "", "", "", err
	}
	seedPhrase, err = s.openField(sealedSeed)
	if err != nil {
		return "", "", "", err
	}
	return npub, nsec, seedPhrase, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

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

// CreateGroup atomically creates the group channel, the admin membership and
// all member rows in one transaction (P4: no half-created groups).
func (s *Store) CreateGroup(name, creator string, members []string) (groupID string, err error) {
	groupID = fmt.Sprintf("grp-%d", time.Now().UnixNano())
	now := time.Now().Unix()
	total := 1
	err = s.RunInTx(context.Background(), func(tx *sql.Tx) error {
		if _, e := tx.Exec(
			`INSERT OR REPLACE INTO channels (id, name, description, creator, subscribers, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			groupID, name, "Group chat", creator, total, now,
		); e != nil {
			return fmt.Errorf("create channel: %w", e)
		}
		if _, e := tx.Exec(
			`INSERT OR REPLACE INTO group_members (group_id, user_npub, role, joined_at) VALUES (?, ?, ?, ?)`,
			groupID, creator, "admin", now,
		); e != nil {
			return fmt.Errorf("create admin member: %w", e)
		}
		for _, m := range members {
			if m == creator || m == "" {
				continue
			}
			if _, e := tx.Exec(
				`INSERT OR REPLACE INTO group_members (group_id, user_npub, role, joined_at) VALUES (?, ?, ?, ?)`,
				groupID, m, "member", now,
			); e != nil {
				return fmt.Errorf("create member %s: %w", m, e)
			}
			total++
		}
		if _, e := tx.Exec(`UPDATE channels SET subscribers = ? WHERE id = ?`, total, groupID); e != nil {
			return fmt.Errorf("update subscriber count: %w", e)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return groupID, nil
}
