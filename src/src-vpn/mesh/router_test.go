package mesh

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Router Tests — BFS route finding with various topologies
// ---------------------------------------------------------------------------

func TestRouter_LineTopology(t *testing.T) {
	t.Parallel()

	// A → B → C → D → E
	m := NewMeshNet(10)
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A", "C"}})
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"B", "D"}})
	m.AddPeer(PeerInfo{ID: "D", Peers: []string{"C", "E"}})
	m.AddPeer(PeerInfo{ID: "E", Peers: []string{"D"}})

	route := m.FindRoute("A", "E")
	if route == nil {
		t.Fatal("expected route from A to E")
	}
	if len(route) != 5 {
		t.Fatalf("expected route length 5 (A→B→C→D→E), got %d: %v", len(route), route)
	}

	// Verify it's the correct path
	expected := []string{"A", "B", "C", "D", "E"}
	for i, id := range expected {
		if route[i] != id {
			t.Fatalf("at position %d: expected %s, got %s", i, id, route[i])
		}
	}
}

func TestRouter_MeshTopology(t *testing.T) {
	t.Parallel()

	// Full mesh: all peers connected to all others
	m := NewMeshNet(6)
	peers := []string{"A", "B", "C", "D"}
	for _, id := range peers {
		var connected []string
		for _, other := range peers {
			if other != id {
				connected = append(connected, other)
			}
		}
		m.AddPeer(PeerInfo{ID: id, Peers: connected})
	}

	// Any route should be direct (length 2)
	route := m.FindRoute("A", "D")
	if route == nil {
		t.Fatal("expected route")
	}
	if len(route) != 2 {
		t.Fatalf("expected direct route (length 2), got %d: %v", len(route), route)
	}
}

func TestRouter_StarTopology(t *testing.T) {
	t.Parallel()

	// Hub-spoke: H connected to A, B, C, D
	m := NewMeshNet(6)
	m.AddPeer(PeerInfo{ID: "H", Peers: []string{"A", "B", "C", "D"}})
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"H"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"H"}})
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"H"}})
	m.AddPeer(PeerInfo{ID: "D", Peers: []string{"H"}})

	// A to D should go through H
	route := m.FindRoute("A", "D")
	if route == nil {
		t.Fatal("expected route from A to D via H")
	}
	if len(route) != 3 {
		t.Fatalf("expected route A→H→D (length 3), got %d: %v", len(route), route)
	}
	if route[0] != "A" || route[1] != "H" || route[2] != "D" {
		t.Fatalf("unexpected route: %v", route)
	}
}

func TestRouter_RingTopology(t *testing.T) {
	t.Parallel()

	// Ring: A → B → C → D → A
	m := NewMeshNet(10)
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B", "D"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A", "C"}})
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"B", "D"}})
	m.AddPeer(PeerInfo{ID: "D", Peers: []string{"C", "A"}})

	// A to C should find shortest path (A→B→C or A→D→C, both length 3)
	route := m.FindRoute("A", "C")
	if route == nil {
		t.Fatal("expected route from A to C")
	}
	if len(route) != 3 {
		t.Fatalf("expected route length 3, got %d: %v", len(route), route)
	}
	if route[0] != "A" || route[2] != "C" {
		t.Fatalf("unexpected route endpoints: %v", route)
	}
}

func TestRouter_PartitionedNetwork(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	// Partition 1: A ↔ B
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A"}})

	// Partition 2: C ↔ D ↔ E
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"D"}})
	m.AddPeer(PeerInfo{ID: "D", Peers: []string{"C", "E"}})
	m.AddPeer(PeerInfo{ID: "E", Peers: []string{"D"}})

	// No route across partitions
	if route := m.FindRoute("A", "E"); route != nil {
		t.Fatalf("expected no route across partitions, got %v", route)
	}

	// Routes within partition work
	if route := m.FindRoute("A", "B"); route == nil {
		t.Fatal("expected route within partition 1")
	}
	if route := m.FindRoute("C", "E"); route == nil {
		t.Fatal("expected route within partition 2")
	}
}

func TestRouter_DynamicPeerAdd(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	// Start with A-B
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A"}})

	// No route to C yet
	if route := m.FindRoute("A", "C"); route != nil {
		t.Fatalf("expected no route to C, got %v", route)
	}

	// Add C connected to B
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A", "C"}}) // update B's peers

	// Now route should exist
	route := m.FindRoute("A", "C")
	if route == nil {
		t.Fatal("expected route A→B→C after adding C")
	}
	if len(route) != 3 {
		t.Fatalf("expected route length 3, got %d: %v", len(route), route)
	}
}

func TestRouter_DynamicPeerRemove(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A", "C"}})
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"B"}})

	// Route exists
	route := m.FindRoute("A", "C")
	if route == nil {
		t.Fatal("expected route before removal")
	}

	// Remove B (the bridge)
	m.RemovePeer("B")

	// No route now
	if route := m.FindRoute("A", "C"); route != nil {
		t.Fatalf("expected no route after removing bridge, got %v", route)
	}
}

func TestRouter_ConcurrentRouteLookup(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(10)
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B", "C"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A", "D"}})
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"A", "D"}})
	m.AddPeer(PeerInfo{ID: "D", Peers: []string{"B", "C", "E"}})
	m.AddPeer(PeerInfo{ID: "E", Peers: []string{"D"}})

	// Run many concurrent route lookups
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			route := m.FindRoute("A", "E")
			done <- route != nil
		}()
	}

	successes := 0
	for i := 0; i < 100; i++ {
		if <-done {
			successes++
		}
	}

	if successes != 100 {
		t.Fatalf("expected 100 successful route lookups, got %d", successes)
	}
}

func TestRouter_GossipDelivery(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)
	m.AddPeer(PeerInfo{ID: "A"})
	m.AddPeer(PeerInfo{ID: "B"})
	m.AddPeer(PeerInfo{ID: "C"})
	m.AddPeer(PeerInfo{ID: "D"})

	// Gossip from A should reach B, C, D
	targets := m.GossipMessage("A", []byte("test"), 3)
	if len(targets) != 3 {
		t.Fatalf("expected 3 gossip targets, got %d", len(targets))
	}

	// Verify A is excluded
	for _, id := range targets {
		if id == "A" {
			t.Fatal("sender should not be in gossip targets")
		}
	}
}

func TestRouter_RouteCacheConsistency(t *testing.T) {
	t.Parallel()

	m := NewMeshNet(6)

	// Build network
	m.AddPeer(PeerInfo{ID: "A", Peers: []string{"B"}})
	m.AddPeer(PeerInfo{ID: "B", Peers: []string{"A", "C"}})
	m.AddPeer(PeerInfo{ID: "C", Peers: []string{"B", "D"}})
	m.AddPeer(PeerInfo{ID: "D", Peers: []string{"C"}})

	// Find route multiple times — should be consistent
	for i := 0; i < 10; i++ {
		route := m.FindRoute("A", "D")
		if route == nil {
			t.Fatalf("route lookup %d failed", i)
		}
		if len(route) != 4 {
			t.Fatalf("route lookup %d: expected length 4, got %d", i, len(route))
		}
	}
}
