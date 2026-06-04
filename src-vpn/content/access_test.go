package content

import "testing"

func TestAccessPublic(t *testing.T) {
	am := NewAccessManager()
	am.SetPolicy(AccessPolicy{
		ContentID: "pub-001",
		Level:     AccessPublic,
	})

	// Anyone can access public content
	if err := am.CheckAccess("pub-001", "anyone", false); err != nil {
		t.Errorf("public access denied for 'anyone': %v", err)
	}
	if err := am.CheckAccess("pub-001", "bob", false); err != nil {
		t.Errorf("public access denied for 'bob': %v", err)
	}
	if err := am.CheckAccess("pub-001", "", false); err != nil {
		t.Errorf("public access denied for empty userID: %v", err)
	}
}

func TestAccessPrivate(t *testing.T) {
	am := NewAccessManager()
	am.SetPolicy(AccessPolicy{
		ContentID:    "priv-001",
		Level:        AccessPrivate,
		AllowedUsers: []string{"alice", "bob"},
	})

	// Allowed users
	if err := am.CheckAccess("priv-001", "alice", false); err != nil {
		t.Errorf("alice should have access: %v", err)
	}
	if err := am.CheckAccess("priv-001", "bob", false); err != nil {
		t.Errorf("bob should have access: %v", err)
	}

	// Non-allowed user
	if err := am.CheckAccess("priv-001", "charlie", false); err == nil {
		t.Error("charlie should not have access")
	}
}

func TestAccessPaid(t *testing.T) {
	am := NewAccessManager()
	am.SetPolicy(AccessPolicy{
		ContentID:    "paid-001",
		Level:        AccessPaid,
		AllowedUsers: []string{"alice"},
		Price:        1000,
	})

	// Premium user gets access regardless of allowed list
	if err := am.CheckAccess("paid-001", "bob", true); err != nil {
		t.Errorf("premium bob should have access: %v", err)
	}

	// Allowed user gets access even without premium
	if err := am.CheckAccess("paid-001", "alice", false); err != nil {
		t.Errorf("allowed alice should have access: %v", err)
	}

	// Non-premium, non-allowed user denied
	if err := am.CheckAccess("paid-001", "charlie", false); err == nil {
		t.Error("non-premium charlie should be denied")
	}
}

func TestAccessDenied(t *testing.T) {
	am := NewAccessManager()
	am.SetPolicy(AccessPolicy{
		ContentID:    "denied-001",
		Level:        AccessPrivate,
		AllowedUsers: []string{"alice"},
	})

	// Not in allowed list
	if err := am.CheckAccess("denied-001", "eve", false); err == nil {
		t.Error("eve should be denied")
	}

	// No policy at all
	if err := am.CheckAccess("nonexistent", "alice", false); err == nil {
		t.Error("nonexistent content should return error")
	}
}
