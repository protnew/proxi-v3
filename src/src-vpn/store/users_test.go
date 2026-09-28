package store

import "testing"

func TestSaveAndGetUser(t *testing.T) {
	s, _ := NewStore(":memory:")
	defer s.Close()

	user := User{
		ID:       "u1",
		Npub:     "alice_npub_123",
		Username: "Alice",
	}
	if err := s.SaveUser(user); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}

	got, err := s.GetUserByNpub("alice_npub_123")
	if err != nil {
		t.Fatalf("GetUserByNpub: %v", err)
	}
	if got.Username != "Alice" {
		t.Errorf("expected 'Alice', got %q", got.Username)
	}
}

func TestGetUserByID(t *testing.T) {
	s, _ := NewStore(":memory:")
	defer s.Close()

	s.SaveUser(User{ID: "u2", Npub: "bob_npub", Username: "Bob"})

	got, err := s.GetUserByID("u2")
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Username != "Bob" {
		t.Errorf("expected 'Bob', got %q", got.Username)
	}
}

func TestUpdateUser(t *testing.T) {
	s, _ := NewStore(":memory:")
	defer s.Close()

	s.SaveUser(User{ID: "u3", Npub: "carol_npub", Username: "Carol"})
	s.SaveUser(User{ID: "u3", Npub: "carol_npub", Username: "Carol Updated"})

	got, _ := s.GetUserByID("u3")
	if got.Username != "Carol Updated" {
		t.Errorf("expected 'Carol Updated', got %q", got.Username)
	}
}

func TestGetNonExistentUser(t *testing.T) {
	s, _ := NewStore(":memory:")
	defer s.Close()

	_, err := s.GetUserByNpub("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

func TestSaveIdentity(t *testing.T) {
	s, _ := NewStore(":memory:")
	defer s.Close()

	err := s.SaveIdentity("npub_test", "nsec_test", "seed phrase words here")
	if err != nil {
		t.Fatalf("SaveIdentity: %v", err)
	}
}

func TestLoadIdentity(t *testing.T) {
	s, _ := NewStore(":memory:")
	defer s.Close()

	s.SaveIdentity("npub_test2", "nsec_test2", "seed phrase")

	npub, nsec, seed, err := s.LoadIdentity()
	if err != nil {
		t.Fatalf("LoadIdentity: %v", err)
	}
	if npub != "npub_test2" {
		t.Errorf("expected npub_test2, got %q", npub)
	}
	if nsec != "nsec_test2" {
		t.Errorf("expected nsec_test2, got %q", nsec)
	}
	if seed != "seed phrase" {
		t.Errorf("expected 'seed phrase', got %q", seed)
	}
}

func TestGroupMembers(t *testing.T) {
	s, _ := NewStore(":memory:")
	defer s.Close()

	gm := GroupMember{GroupID: "g1", UserNpub: "alice_npub", Role: "admin"}
	if err := s.SaveGroupMember(gm); err != nil {
		t.Fatalf("SaveGroupMember: %v", err)
	}

	members, err := s.GetGroupMembers("g1")
	if err != nil {
		t.Fatalf("GetGroupMembers: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}

	isAdmin, err := s.IsGroupAdmin("g1", "alice_npub")
	if err != nil {
		t.Fatalf("IsGroupAdmin: %v", err)
	}
	if !isAdmin {
		t.Error("expected alice to be admin")
	}

	if err := s.UpdateGroupMemberRole("g1", "alice_npub", "member"); err != nil {
		t.Fatalf("UpdateGroupMemberRole: %v", err)
	}

	if err := s.RemoveGroupMember("g1", "alice_npub"); err != nil {
		t.Fatalf("RemoveGroupMember: %v", err)
	}
}

func TestPreKeyBundle(t *testing.T) {
	s, _ := NewStore(":memory:")
	defer s.Close()

	err := s.StorePreKeyBundle("user1",
		[]byte("identity_key_32bytes___________"),
		[]byte("signed_pre_key_32bytes________"),
		[]byte("signature_64bytes_______________________________"),
		[]byte("one_time_pre_key_32bytes_______"),
	)
	if err != nil {
		t.Fatalf("StorePreKeyBundle: %v", err)
	}

	got, err := s.GetPreKeyBundle("user1")
	if err != nil {
		t.Fatalf("GetPreKeyBundle: %v", err)
	}
	if string(got.IdentityKey) != "identity_key_32bytes___________" {
		t.Error("identity key mismatch")
	}
}
