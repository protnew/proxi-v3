package chat

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// GroupPermissions defines what members can do in a group.
type GroupPermissions struct {
	CanPost         bool
	CanInvite       bool
	CanPin          bool
	SlowModeSeconds int
}

// groupState tracks per-group permissions and membership roles.
type groupState struct {
	// default permissions for regular members
	defaultPerms GroupPermissions
	// per-member overrides: userID → GroupPermissions
	memberPerms map[string]GroupPermissions
	// admins: userID → true
	admins map[string]bool
}

var (
	groupsMu   sync.RWMutex
	groupsStore = make(map[string]*groupState)

	inviteMu   sync.RWMutex
	inviteCode = make(map[string]string) // groupID → code
)

// DefaultGroupPermissions returns the standard permission set for new groups.
func DefaultGroupPermissions() GroupPermissions {
	return GroupPermissions{
		CanPost:         true,
		CanInvite:       true,
		CanPin:          false,
		SlowModeSeconds: 0,
	}
}

// SetGroupPermissions initialises group state with the given default permissions.
// If the group already exists its defaults are updated.
func SetGroupPermissions(groupID string, perms GroupPermissions) {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	gs, ok := groupsStore[groupID]
	if !ok {
		gs = &groupState{
			defaultPerms: perms,
			memberPerms:  make(map[string]GroupPermissions),
			admins:       make(map[string]bool),
		}
		groupsStore[groupID] = gs
	} else {
		gs.defaultPerms = perms
	}
}

// SetGroupAdmin marks (or unmarks) a user as an admin for the group.
func SetGroupAdmin(groupID, userID string, isAdmin bool) {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	gs, ok := groupsStore[groupID]
	if !ok {
		gs = &groupState{
			defaultPerms: DefaultGroupPermissions(),
			memberPerms:  make(map[string]GroupPermissions),
			admins:       make(map[string]bool),
		}
		groupsStore[groupID] = gs
	}
	if isAdmin {
		gs.admins[userID] = true
	} else {
		delete(gs.admins, userID)
	}
}

// SetMemberPermissions sets custom permissions for a specific group member.
func SetMemberPermissions(groupID, userID string, perms GroupPermissions) {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	gs, ok := groupsStore[groupID]
	if !ok {
		gs = &groupState{
			defaultPerms: DefaultGroupPermissions(),
			memberPerms:  make(map[string]GroupPermissions),
			admins:       make(map[string]bool),
		}
		groupsStore[groupID] = gs
	}
	gs.memberPerms[userID] = perms
}

// CheckPermission returns true if the user has the specified permission in the group.
// Admins always get true for any permission.
// Per-member overrides take precedence over group defaults.
// Supported perm strings: "post", "invite", "pin".
func CheckPermission(groupID, userID string, perm string) bool {
	groupsMu.RLock()
	defer groupsMu.RUnlock()

	gs, ok := groupsStore[groupID]
	if !ok {
		return false
	}

	// Admins have all permissions
	if gs.admins[userID] {
		return true
	}

	// Check per-member override first
	var p GroupPermissions
	if mp, has := gs.memberPerms[userID]; has {
		p = mp
	} else {
		p = gs.defaultPerms
	}

	switch perm {
	case "post":
		return p.CanPost
	case "invite":
		return p.CanInvite
	case "pin":
		return p.CanPin
	default:
		return false
	}
}

// GenerateInviteLink creates a unique invite link for a group.
// Format: https://proxi.app/invite/{code}
func GenerateInviteLink(groupID string) string {
	inviteMu.Lock()
	defer inviteMu.Unlock()

	// Return existing code if already generated
	if code, ok := inviteCode[groupID]; ok {
		return fmt.Sprintf("https://proxi.app/invite/%s", code)
	}

	// Generate 16 random bytes → 32 hex chars
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: should never happen
		code := fmt.Sprintf("%d", len(inviteCode)+1)
		inviteCode[groupID] = code
		return fmt.Sprintf("https://proxi.app/invite/%s", code)
	}
	code := hex.EncodeToString(b)
	inviteCode[groupID] = code

	return fmt.Sprintf("https://proxi.app/invite/%s", code)
}
