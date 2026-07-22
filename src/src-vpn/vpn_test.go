package vpn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "vpn-test")
}

func TestNewManager(t *testing.T) {
	dir := tempDir(t)
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	defer os.RemoveAll(dir)

	pubKey := mgr.GetPublicKey()
	if pubKey == "" {
		t.Error("manager should have a non-empty public key")
	}
	if len(pubKey) < 20 {
		t.Errorf("public key seems too short: %q", pubKey)
	}
}

func TestGetStatus(t *testing.T) {
	dir := tempDir(t)
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	status := mgr.GetStatus()
	if status.State != StateDisconnected {
		t.Errorf("initial state = %q, want %q", status.State, StateDisconnected)
	}
	if status.MyPubKey == "" {
		t.Error("status should contain public key")
	}
}

func TestAddPeer(t *testing.T) {
	dir := tempDir(t)
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	longKey := "abcdefghijklmnopqrstuvwxyz1234567890ABCDEF"
	if err := mgr.AddPeer("test-peer", longKey, "1.2.3.4:51820"); err != nil {
		t.Fatalf("AddPeer: %v", err)
	}

	status := mgr.GetStatus()
	if len(status.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(status.Peers))
	}
	if status.Peers[0].Name != "test-peer" {
		t.Errorf("peer name = %q, want %q", status.Peers[0].Name, "test-peer")
	}

	// Remove peer
	peerID := longKey[:16]
	if err := mgr.RemovePeer(peerID); err != nil {
		t.Fatalf("RemovePeer: %v", err)
	}
	status = mgr.GetStatus()
	if len(status.Peers) != 0 {
		t.Errorf("expected 0 peers after remove, got %d", len(status.Peers))
	}
}

func TestAddPeerShortKey(t *testing.T) {
	dir := tempDir(t)
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = mgr.AddPeer("bad-peer", "short", "1.2.3.4:51820")
	if err == nil {
		t.Error("expected error for short public key")
	}
}

func TestVPNStatusJSON(t *testing.T) {
	dir := tempDir(t)
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	status := mgr.GetStatus()
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal(Status): %v", err)
	}

	// Parse back
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if parsed["state"] != string(StateDisconnected) {
		t.Errorf("state = %v, want %q", parsed["state"], StateDisconnected)
	}
	if _, ok := parsed["myPublicKey"]; !ok {
		t.Error("status JSON should contain myPublicKey")
	}
	if _, ok := parsed["peers"]; !ok {
		t.Error("status JSON should contain peers")
	}
}
