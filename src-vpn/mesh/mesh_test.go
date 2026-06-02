package mesh

import (
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestNewMeshNet
// ---------------------------------------------------------------------------

func TestNewMeshNet(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(8)
	if m == nil {
		t.Fatal("NewMeshNet returned nil")
	}
	if m.maxHops != 8 {
		t.Fatalf("expected maxHops=8, got %d", m.maxHops)
	}
	if m.peers == nil || m.knownIDs == nil {
		t.Fatal("internal maps should be initialized")
	}
}

func TestNewMeshNet_DefaultMaxHops(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(0)
	if m.maxHops != 6 {
		t.Fatalf("expected default maxHops=6 for zero, got %d", m.maxHops)
	}

	m2 := NewMeshNet(-1)
	if m2.maxHops != 6 {
		t.Fatalf("expected default maxHops=6 for negative, got %d", m2.maxHops)
	}
}

// ---------------------------------------------------------------------------
// TestAddPeer
// ---------------------------------------------------------------------------

func TestAddPeer(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	p := PeerInfo{
		ID:        "peer1",
		PublicKey: "pk1",
		Address:   "addr1",
		Peers:     []string{"peer2", "peer3"},
	}
	m.AddPeer(p)

	got := m.GetPeer("peer1")
	if got == nil {
		t.Fatal("peer1 should exist")
	}
	if got.PublicKey != "pk1" {
		t.Fatalf("expected publicKey=pk1, got %s", got.PublicKey)
	}
	if got.Address != "addr1" {
		t.Fatalf("expected address=addr1, got %s", got.Address)
	}
	if got.LastSeen == 0 {
		t.Fatal("LastSeen should be set")
	}

	// Known IDs should include peer1 and its known peers (peer2, peer3).
	if m.KnownPeerCount() != 3 {
		t.Fatalf("expected 3 known IDs, got %d", m.KnownPeerCount())
	}
}

func TestAddPeer_UpdateExisting(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	m.AddPeer(PeerInfo{ID: "p1", PublicKey: "pk_old"})
	m.AddPeer(PeerInfo{ID: "p1", PublicKey: "pk_new"})

	got := m.GetPeer("p1")
	if got.PublicKey != "pk_new" {
		t.Fatalf("expected updated publicKey=pk_new, got %s", got.PublicKey)
	}
}

// ---------------------------------------------------------------------------
// TestRemovePeer
// ---------------------------------------------------------------------------

func TestRemovePeer(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	m.AddPeer(PeerInfo{ID: "p1", PublicKey: "pk1"})
	m.AddPeer(PeerInfo{ID: "p2", PublicKey: "pk2"})

	if m.DirectPeerCount() != 2 {
		t.Fatalf("expected 2 peers, got %d", m.DirectPeerCount())
	}

	m.RemovePeer("p1")

	if m.GetPeer("p1") != nil {
		t.Fatal("p1 should be removed")
	}
	if m.DirectPeerCount() != 1 {
		t.Fatalf("expected 1 peer after removal, got %d", m.DirectPeerCount())
	}

	// Remove non-existent — should not panic.
	m.RemovePeer("nonexistent")
}

// ---------------------------------------------------------------------------
// TestGetPeer
// ---------------------------------------------------------------------------

func TestGetPeer(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	// Non-existent peer.
	if p := m.GetPeer("nope"); p != nil {
		t.Fatal("expected nil for non-existent peer")
	}

	m.AddPeer(PeerInfo{ID: "abc", PublicKey: "pk_abc", Address: "10.0.0.1"})
	p := m.GetPeer("abc")
	if p == nil {
		t.Fatal("expected peer, got nil")
	}
	if p.Address != "10.0.0.1" {
		t.Fatalf("expected address 10.0.0.1, got %s", p.Address)
	}
}

// ---------------------------------------------------------------------------
// TestGetAllPeers
// ---------------------------------------------------------------------------

func TestGetAllPeers(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	// Empty.
	if all := m.GetAllPeers(); len(all) != 0 {
		t.Fatalf("expected empty, got %d", len(all))
	}

	m.AddPeer(PeerInfo{ID: "p1"})
	m.AddPeer(PeerInfo{ID: "p2"})
	m.AddPeer(PeerInfo{ID: "p3"})

	all := m.GetAllPeers()
	if len(all) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(all))
	}

	// Verify all IDs are present.
	ids := map[string]bool{}
	for _, p := range all {
		ids[p.ID] = true
	}
	for _, expected := range []string{"p1", "p2", "p3"} {
		if !ids[expected] {
			t.Fatalf("missing peer %s", expected)
		}
	}
}

// ---------------------------------------------------------------------------
// TestFindRoute_Simple — direct peer connection
// ---------------------------------------------------------------------------

func TestFindRoute_Simple(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	// A knows B directly.
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A"}})

	route := m.FindRoute("A", "B")
	if route == nil {
		t.Fatal("expected a route from A to B")
	}
	if len(route) != 2 {
		t.Fatalf("expected route length 2 (A→B), got %d: %v", len(route), route)
	}
	if route[0] != "A" || route[1] != "B" {
		t.Fatalf("unexpected route: %v", route)
	}
}

// ---------------------------------------------------------------------------
// TestFindRoute_MultiHop — A → B → C
// ---------------------------------------------------------------------------

func TestFindRoute_MultiHop(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A", "C"}})
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"B"}})

	route := m.FindRoute("A", "C")
	if route == nil {
		t.Fatal("expected a route from A to C")
	}
	if len(route) != 3 {
		t.Fatalf("expected route A→B→C (length 3), got %d: %v", len(route), route)
	}
	if route[0] != "A" || route[1] != "B" || route[2] != "C" {
		t.Fatalf("unexpected route: %v", route)
	}
}

// ---------------------------------------------------------------------------
// TestFindRoute_NoRoute — disconnected components
// ---------------------------------------------------------------------------

func TestFindRoute_NoRoute(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A"}})
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"D"}})
	m.AddPeer(PeerInfo{ID: "D", Peers: []string{"C"}})

	route := m.FindRoute("A", "D")
	if route != nil {
		t.Fatalf("expected no route, got %v", route)
	}
}

// ---------------------------------------------------------------------------
// TestFindRoute_SelfRoute — from == to
// ---------------------------------------------------------------------------

func TestFindRoute_SelfRoute(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})

	route := m.FindRoute("A", "A")
	if route != nil {
		t.Fatalf("self-route should return nil, got %v", route)
	}
}

// ---------------------------------------------------------------------------
// TestFindRoute_MaxHopsExceeded
// ---------------------------------------------------------------------------

func TestFindRoute_MaxHopsExceeded(t *testing.T) {
	t.Parallel()

	// Very low maxHops.
	m := NewMeshNet(2)

	// A → B → C → D  (needs 3 hops, maxHops=2).
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A", "C"}})
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"B", "D"}})
	m.AddPeer(PeerInfo{ID: "D", Peers: []string{"C"}})

	route := m.FindRoute("A", "D")
	if route != nil {
		t.Fatalf("expected no route (exceeds maxHops), got %v", route)
	}
}

// ---------------------------------------------------------------------------
// TestGossipMessage
// ---------------------------------------------------------------------------

func TestGossipMessage(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	// No peers → empty targets.
	targets := m.GossipMessage("", []byte("hi"), 3)
	if len(targets) != 0 {
		t.Fatalf("expected 0 targets, got %d", len(targets))
	}

	m.AddPeer(PeerInfo{ID: "A"})
	m.AddPeer(PeerInfo{ID: "B"})
	m.AddPeer(PeerInfo{ID: "C"})

	// Gossip from "A" should target B and C.
	targets = m.GossipMessage("A", []byte("hello"), 2)

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), targets)
	}

	for _, id := range targets {
		if id == "A" {
			t.Fatal("gossip should exclude the sender")
		}
	}

	// Verify B and C are both in targets.
	set := map[string]bool{}
	for _, id := range targets {
		set[id] = true
	}
	if !set["B"] || !set["C"] {
		t.Fatalf("expected B and C as targets, got %v", targets)
	}
}

func TestGossipMessage_DefaultTTL(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)
	m.AddPeer(PeerInfo{ID: "X"})
	m.AddPeer(PeerInfo{ID: "Y"})

	// ttl=0 should use maxHops as default.
	targets := m.GossipMessage("X", []byte("msg"), 0)
	if len(targets) != 1 || targets[0] != "Y" {
		t.Fatalf("expected [Y], got %v", targets)
	}
}

// ---------------------------------------------------------------------------
// TestPruneStale
// ---------------------------------------------------------------------------

func TestPruneStale(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	// Add a peer and manually set LastSeen to the past.
	m.AddPeer(PeerInfo{ID: "old_peer_1234", PublicKey: "pk_old"})
	m.AddPeer(PeerInfo{ID: "fresh_peer_5678", PublicKey: "pk_fresh"})

	// Force the "old" peer's LastSeen to 2 hours ago.
	m.mu.Lock()
	p := m.peers["old_peer_1234"]
	p.LastSeen = time.Now().Add(-2 * time.Hour).Unix()
	m.mu.Unlock()

	// Prune peers older than 1 hour.
	m.PruneStale(1 * time.Hour)

	// "old" should be removed.
	if m.GetPeer("old_peer_1234") != nil {
		t.Fatal("old peer should have been pruned")
	}

	// "fresh" should still be there.
	if m.GetPeer("fresh_peer_5678") == nil {
		t.Fatal("fresh peer should still exist")
	}

	if m.KnownPeerCount() != 1 {
		t.Fatalf("expected 1 known ID, got %d", m.KnownPeerCount())
	}
}

func TestPruneStale_NoneStale(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)
	m.AddPeer(PeerInfo{ID: "p1"})
	m.AddPeer(PeerInfo{ID: "p2"})

	// Prune with a very large maxAge — nothing should be removed.
	m.PruneStale(365 * 24 * time.Hour)

	if m.DirectPeerCount() != 2 {
		t.Fatalf("expected 2 peers, got %d", m.DirectPeerCount())
	}
}

// ---------------------------------------------------------------------------
// TestGetStats
// ---------------------------------------------------------------------------

func TestGetStats(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(8)

	// Empty mesh.
	stats := m.GetStats()
	if stats["directPeers"] != 0 {
		t.Fatalf("expected 0 directPeers, got %v", stats["directPeers"])
	}
	if stats["knownPeers"] != 0 {
		t.Fatalf("expected 0 knownPeers, got %v", stats["knownPeers"])
	}
	if stats["maxHops"] != 8 {
		t.Fatalf("expected maxHops=8, got %v", stats["maxHops"])
	}

	// Add peers.
	m.AddPeer(PeerInfo{ID: "p1", Peers: []string{"p2", "p3"}})
	m.AddPeer(PeerInfo{ID: "p2", Peers: []string{"p1"}})

	stats = m.GetStats()

	if stats["directPeers"] != 2 {
		t.Fatalf("expected 2 directPeers, got %v", stats["directPeers"])
	}
	// p1, p2 are direct; p3 is known via p1's Peers list.
	if stats["knownPeers"] != 3 {
		t.Fatalf("expected 3 knownPeers, got %v", stats["knownPeers"])
	}
	if stats["maxHops"] != 8 {
		t.Fatalf("expected maxHops=8, got %v", stats["maxHops"])
	}

	avgConnectivity := stats["avgConnectivity"].(float64)
	// p1 has 2 peers, p2 has 1 peer → (2+1)/2 = 1.5
	if avgConnectivity != 1.5 {
		t.Fatalf("expected avgConnectivity=1.5, got %v", avgConnectivity)
	}
}

// ---------------------------------------------------------------------------
// TestMarshalPeers
// ---------------------------------------------------------------------------

func TestMarshalPeers(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	// Empty.
	data := m.MarshalPeers()
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty, got %v", ids)
	}

	m.AddPeer(PeerInfo{ID: "x"})
	m.AddPeer(PeerInfo{ID: "y"})

	data = m.MarshalPeers()
	if err := json.Unmarshal(data, &ids); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}

	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	if !set["x"] || !set["y"] {
		t.Fatalf("expected x and y, got %v", ids)
	}
}

// ---------------------------------------------------------------------------
// TestDirectPeerCount / TestKnownPeerCount
// ---------------------------------------------------------------------------

func TestDirectPeerCount(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)
	if m.DirectPeerCount() != 0 {
		t.Fatal("expected 0")
	}
	m.AddPeer(PeerInfo{ID: "a"})
	if m.DirectPeerCount() != 1 {
		t.Fatal("expected 1")
	}
	m.RemovePeer("a")
	if m.DirectPeerCount() != 0 {
		t.Fatal("expected 0 after removal")
	}
}

func TestKnownPeerCount(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)
	m.AddPeer(PeerInfo{ID: "a", Peers: []string{"b", "c"}})

	// Direct: a; Known: a, b, c
	if m.DirectPeerCount() != 1 {
		t.Fatalf("expected 1 direct, got %d", m.DirectPeerCount())
	}
	if m.KnownPeerCount() != 3 {
		t.Fatalf("expected 3 known, got %d", m.KnownPeerCount())
	}
}
