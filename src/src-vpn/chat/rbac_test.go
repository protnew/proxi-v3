package chat

import (
	"testing"
)

func TestRBAC_AdminAlwaysHasFullPermissions(t *testing.T) {
	groupID := "test-rbac-group-1"
	adminID := "admin-user"

	// Restrict defaults
	SetGroupPermissions(groupID, GroupPermissions{
		CanPost:   false,
		CanInvite: false,
		CanPin:    false,
	})

	// Make user admin
	SetGroupAdmin(groupID, adminID, true)

	if !CheckPermission(groupID, adminID, "post") {
		t.Errorf("Admin should always be able to post")
	}
	if !CheckPermission(groupID, adminID, "pin") {
		t.Errorf("Admin should always be able to pin")
	}
}

func TestRBAC_UserCustomPermissionsOverrideDefaults(t *testing.T) {
	groupID := "test-rbac-group-2"
	userID := "regular-user"

	// Defaults allow posting, but not pinning
	SetGroupPermissions(groupID, GroupPermissions{
		CanPost:   true,
		CanInvite: true,
		CanPin:    false,
	})

	// Give custom permissions to user: deny post, allow pin
	SetMemberPermissions(groupID, userID, GroupPermissions{
		CanPost:   false,
		CanInvite: false,
		CanPin:    true,
	})

	if CheckPermission(groupID, userID, "post") {
		t.Errorf("Expected post permission to be denied by custom override")
	}
	if !CheckPermission(groupID, userID, "pin") {
		t.Errorf("Expected pin permission to be granted by custom override")
	}
}

func TestRBAC_DefaultPermissionsApply(t *testing.T) {
	groupID := "test-rbac-group-3"
	userID := "new-user"

	SetGroupPermissions(groupID, GroupPermissions{
		CanPost:   true,
		CanInvite: false,
		CanPin:    false,
	})

	if !CheckPermission(groupID, userID, "post") {
		t.Errorf("Expected post permission from defaults")
	}
	if CheckPermission(groupID, userID, "invite") {
		t.Errorf("Expected invite permission to be denied from defaults")
	}
}
