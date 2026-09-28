package chat

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestGroupPermissions — default, custom per-member, admin override
// ---------------------------------------------------------------------------

func TestGroupPermissions(t *testing.T) {
	t.Parallel()

	groupID := "grp-perm-test"

	// Set up group with default permissions
	SetGroupPermissions(groupID, GroupPermissions{
		CanPost:   true,
		CanInvite: true,
		CanPin:    false,
	})

	// Default member should be able to post and invite but not pin
	if !CheckPermission(groupID, "user1", "post") {
		t.Fatal("default member should be able to post")
	}
	if !CheckPermission(groupID, "user1", "invite") {
		t.Fatal("default member should be able to invite")
	}
	if CheckPermission(groupID, "user1", "pin") {
		t.Fatal("default member should NOT be able to pin")
	}

	// Set custom per-member permissions (muted user)
	SetMemberPermissions(groupID, "muted", GroupPermissions{
		CanPost:   false,
		CanInvite: false,
		CanPin:    false,
	})
	if CheckPermission(groupID, "muted", "post") {
		t.Fatal("muted user should NOT be able to post")
	}
	if CheckPermission(groupID, "muted", "invite") {
		t.Fatal("muted user should NOT be able to invite")
	}

	// Admin override — admin can do everything even if default denies pin
	SetGroupAdmin(groupID, "admin1", true)
	if !CheckPermission(groupID, "admin1", "post") {
		t.Fatal("admin should be able to post")
	}
	if !CheckPermission(groupID, "admin1", "pin") {
		t.Fatal("admin should be able to pin (override)")
	}
	if !CheckPermission(groupID, "admin1", "invite") {
		t.Fatal("admin should be able to invite")
	}

	// Unknown permission string
	if CheckPermission(groupID, "user1", "unknown") {
		t.Fatal("unknown permission should return false")
	}

	// Unknown group
	if CheckPermission("nonexistent-group", "user1", "post") {
		t.Fatal("unknown group should return false")
	}

	// Clean up
	groupsMu.Lock()
	delete(groupsStore, groupID)
	groupsMu.Unlock()
}

// ---------------------------------------------------------------------------
// TestInviteLink — format and uniqueness
// ---------------------------------------------------------------------------

func TestInviteLink(t *testing.T) {
	t.Parallel()

	link1 := GenerateInviteLink("group-a")
	link2 := GenerateInviteLink("group-b")
	link1Again := GenerateInviteLink("group-a")

	// Format check: must start with https://proxi.app/invite/
	prefix := "https://proxi.app/invite/"
	if !strings.HasPrefix(link1, prefix) {
		t.Fatalf("invite link should start with %s, got %s", prefix, link1)
	}
	if !strings.HasPrefix(link2, prefix) {
		t.Fatalf("invite link should start with %s, got %s", prefix, link2)
	}

	// Code should be 32 hex characters
	code1 := strings.TrimPrefix(link1, prefix)
	if len(code1) != 32 {
		t.Fatalf("invite code should be 32 hex chars, got %d: %s", len(code1), code1)
	}
	for _, c := range code1 {
		if !isHexChar(c) {
			t.Fatalf("invite code should be hex, found '%c'", c)
		}
	}

	// Uniqueness: different groups must yield different codes
	if link1 == link2 {
		t.Fatal("invite links for different groups should differ")
	}

	// Idempotency: same group returns same link
	if link1 != link1Again {
		t.Fatalf("same group should return same link: %s vs %s", link1, link1Again)
	}

	// Clean up
	inviteMu.Lock()
	delete(inviteCode, "group-a")
	delete(inviteCode, "group-b")
	inviteMu.Unlock()
}

// isHexChar checks if a rune is a valid hexadecimal character.
func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
