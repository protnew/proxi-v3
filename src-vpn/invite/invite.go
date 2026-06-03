package invite

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// Invite represents an invitation link for channels or groups.
type Invite struct {
	ID        string `json:"id"`
	Channel   string `json:"channel,omitempty"`
	Group     string `json:"group,omitempty"`
	CreatedBy string `json:"created_by"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	Uses      int    `json:"uses"`
	MaxUses   int    `json:"max_uses,omitempty"`
}

// GenerateID creates a random invite ID.
func GenerateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateInvite creates a new invite link.
func CreateInvite(db *store.Store, channel, group, createdBy string, maxUses int, ttlSeconds int64) (*Invite, error) {
	id := GenerateID()
	code := GenerateID()[:8] // short invite code
	var expiresAt int64
	if ttlSeconds > 0 {
		expiresAt = time.Now().Unix() + ttlSeconds
	}
	createdAt := time.Now().Unix()

	d := db.DB()
	_, err := d.Exec(
		"INSERT INTO invites (id, code, channel, group_name, created_by, expires_at, max_uses, uses, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)",
		id, code, channel, group, createdBy, expiresAt, maxUses, createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create invite: %w", err)
	}

	return &Invite{
		ID:        id,
		Channel:   channel,
		Group:     group,
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
		MaxUses:   maxUses,
		Uses:      0,
	}, nil
}

// GetInvite retrieves an invite by ID.
func GetInvite(db *store.Store, id string) (*Invite, error) {
	d := db.DB()
	inv := &Invite{}
	var channel, group sql.NullString
	err := d.QueryRow(
		"SELECT id, COALESCE(channel,''), COALESCE(group_name,''), created_by, COALESCE(expires_at,0), uses, COALESCE(max_uses,0) FROM invites WHERE id = ?",
		id,
	).Scan(&inv.ID, &channel, &group, &inv.CreatedBy, &inv.ExpiresAt, &inv.Uses, &inv.MaxUses)
	if err != nil {
		return nil, fmt.Errorf("invite not found: %w", err)
	}
	inv.Channel = channel.String
	inv.Group = group.String
	return inv, nil
}

// RedeemInvite uses an invite link. Returns error if expired or max uses reached.
func RedeemInvite(db *store.Store, id string, userID string) error {
	inv, err := GetInvite(db, id)
	if err != nil {
		return err
	}

	// Check expiry
	if inv.ExpiresAt > 0 && time.Now().Unix() > inv.ExpiresAt {
		return fmt.Errorf("invite expired")
	}

	// Check max uses
	if inv.MaxUses > 0 && inv.Uses >= inv.MaxUses {
		return fmt.Errorf("invite max uses reached")
	}

	// Increment uses
	d := db.DB()
	_, err = d.Exec("UPDATE invites SET uses = uses + 1 WHERE id = ?", id)
	return err
}

// GenerateInviteURL creates a full URL for the invite.
func GenerateInviteURL(baseURL, inviteID string) string {
	return fmt.Sprintf("%s/invite/%s", baseURL, inviteID)
}
