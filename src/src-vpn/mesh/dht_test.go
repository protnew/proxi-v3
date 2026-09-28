package mesh

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"testing"
)

func TestRoutingTable_AddFind(t *testing.T) {
	rt := NewRoutingTable("self")

	peer1 := PeerInfo{ID: "peer1", Address: "10.0.0.1:3000"}
	peer2 := PeerInfo{ID: "peer2", Address: "10.0.0.2:3000"}
	peer3 := PeerInfo{ID: "peer3", Address: "10.0.0.3:3000"}

	rt.AddPeer(peer1)
	rt.AddPeer(peer2)
	rt.AddPeer(peer3)

	if rt.Size() != 3 {
		t.Errorf("expected size 3, got %d", rt.Size())
	}

	closest := rt.FindClosest("peer1", 1)
	if len(closest) != 1 {
		t.Fatalf("expected 1 closest peer, got %d", len(closest))
	}
	if closest[0].ID != "peer1" {
		t.Errorf("expected closest peer 'peer1', got '%s'", closest[0].ID)
	}
}

func TestRoutingTable_XORdistance(t *testing.T) {
	// Identical strings should have distance 0
	d := distance("abc", "abc")
	if d != 0 {
		t.Errorf("expected distance 0 for identical strings, got %d", d)
	}

	// Different strings should have non-zero distance
	d = distance("aaa", "bbb")
	if d == 0 {
		t.Error("expected non-zero distance for different strings")
	}

	// XOR distance should be symmetric
	d1 := distance("peer1", "peer2")
	d2 := distance("peer2", "peer1")
	if d1 != d2 {
		t.Errorf("XOR distance should be symmetric: d1=%d, d2=%d", d1, d2)
	}

	// Distance from string to itself should be 0
	d = distance("self", "self")
	if d != 0 {
		t.Errorf("expected distance 0 for same string, got %d", d)
	}

	// Empty strings should have distance 0
	d = distance("", "")
	if d != 0 {
		t.Errorf("expected distance 0 for empty strings, got %d", d)
	}

	// One empty, one non-empty should have non-zero distance
	d = distance("", "abc")
	if d == 0 {
		t.Error("expected non-zero distance between empty and non-empty")
	}
}

func TestRoutingTable_Remove(t *testing.T) {
	rt := NewRoutingTable("self")

	peer1 := PeerInfo{ID: "peer1", Address: "10.0.0.1:3000"}
	peer2 := PeerInfo{ID: "peer2", Address: "10.0.0.2:3000"}

	rt.AddPeer(peer1)
	rt.AddPeer(peer2)

	if rt.Size() != 2 {
		t.Errorf("expected size 2, got %d", rt.Size())
	}

	rt.RemovePeer("peer1")

	if rt.Size() != 1 {
		t.Errorf("expected size 1 after removal, got %d", rt.Size())
	}

	closest := rt.FindClosest("peer1", 1)
	if len(closest) != 1 || closest[0].ID != "peer2" {
		t.Errorf("expected peer2 to remain, got %v", closest)
	}

	// Remove non-existent peer should not panic
	rt.RemovePeer("nonexistent")
	if rt.Size() != 1 {
		t.Errorf("expected size 1 after removing nonexistent, got %d", rt.Size())
	}
}

func TestRoutingTable_FindClosest_Nearest(t *testing.T) {
	rt := NewRoutingTable("self_node")

	// Add peers with IDs that have varying distances from "target"
	peers := []PeerInfo{
		{ID: "target", Address: "10.0.0.1:3000"},    // distance 0 from itself
		{ID: "target_a", Address: "10.0.0.2:3000"},  // close
		{ID: "target_ab", Address: "10.0.0.3:3000"}, // also close
		{ID: "zzzzzzz", Address: "10.0.0.4:3000"},   // far
		{ID: "xxxxxxx", Address: "10.0.0.5:3000"},   // far
	}

	for _, p := range peers {
		rt.AddPeer(p)
	}

	// Find 3 closest to "target"
	closest := rt.FindClosest("target", 3)
	if len(closest) != 3 {
		t.Fatalf("expected 3 closest peers, got %d", len(closest))
	}

	// The first result should be "target" itself (distance 0)
	if closest[0].ID != "target" {
		t.Errorf("expected closest to be 'target', got '%s'", closest[0].ID)
	}

	// Verify sorted by distance
	for i := 1; i < len(closest); i++ {
		d1 := distance(closest[i-1].ID, "target")
		d2 := distance(closest[i].ID, "target")
		if d1 > d2 {
			t.Errorf("results not sorted by distance: %d > %d at index %d", d1, d2, i)
		}
	}
}

func TestRoutingTable_FindClosest_CountZero(t *testing.T) {
	rt := NewRoutingTable("self")
	rt.AddPeer(PeerInfo{ID: "peer1", Address: "10.0.0.1:3000"})

	result := rt.FindClosest("peer1", 0)
	if result != nil {
		t.Errorf("expected nil for count=0, got %v", result)
	}
}

func TestRoutingTable_FindClosest_MoreThanAvailable(t *testing.T) {
	rt := NewRoutingTable("self")
	rt.AddPeer(PeerInfo{ID: "peer1", Address: "10.0.0.1:3000"})
	rt.AddPeer(PeerInfo{ID: "peer2", Address: "10.0.0.2:3000"})

	closest := rt.FindClosest("peer1", 10)
	if len(closest) != 2 {
		t.Errorf("expected 2 peers (all available), got %d", len(closest))
	}
}

func TestRoutingTable_NoSelf(t *testing.T) {
	rt := NewRoutingTable("self")

	// Adding self should be ignored
	selfPeer := PeerInfo{ID: "self", Address: "10.0.0.0:3000"}
	rt.AddPeer(selfPeer)

	if rt.Size() != 0 {
		t.Errorf("expected size 0 (self should not be added), got %d", rt.Size())
	}
}

func TestRoutingTable_Update(t *testing.T) {
	rt := NewRoutingTable("self")

	rt.AddPeer(PeerInfo{ID: "peer1", Address: "10.0.0.1:3000"})
	rt.AddPeer(PeerInfo{ID: "peer1", Address: "10.0.0.99:3000"}) // update

	if rt.Size() != 1 {
		t.Errorf("expected size 1 (update, not add), got %d", rt.Size())
	}

	closest := rt.FindClosest("peer1", 1)
	if len(closest) != 1 {
		t.Fatal("expected 1 peer")
	}
	if closest[0].Address != "10.0.0.99:3000" {
		t.Errorf("expected updated address, got %s", closest[0].Address)
	}
}

func TestRoutingTable_Concurrent(t *testing.T) {
	rt := NewRoutingTable("self")

	var wg sync.WaitGroup
	numGoroutines := 50
	peersPerGoroutine := 20

	// Concurrent AddPeer
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < peersPerGoroutine; i++ {
				id := fmt.Sprintf("peer_%d_%d", g, i)
				rt.AddPeer(PeerInfo{ID: id, Address: fmt.Sprintf("10.%d.%d.1:3000", g%256, i%256)})
			}
		}(g)
	}
	wg.Wait()

	expected := numGoroutines * peersPerGoroutine
	if rt.Size() != expected {
		t.Errorf("expected %d peers, got %d", expected, rt.Size())
	}

	// Concurrent FindClosest
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			closest := rt.FindClosest(fmt.Sprintf("peer_%d_0", g), 5)
			if len(closest) == 0 {
				t.Errorf("expected at least 1 closest peer")
			}
		}(g)
	}
	wg.Wait()

	// Concurrent Remove
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < peersPerGoroutine/2; i++ {
				id := fmt.Sprintf("peer_%d_%d", g, i)
				rt.RemovePeer(id)
			}
		}(g)
	}
	wg.Wait()

	remaining := expected - numGoroutines*(peersPerGoroutine/2)
	if rt.Size() != remaining {
		t.Errorf("expected %d remaining peers, got %d", remaining, rt.Size())
	}
}

func TestRoutingTable_LargeScale(t *testing.T) {
	rt := NewRoutingTable("self")

	numPeers := 1000
	peers := make([]PeerInfo, numPeers)
	for i := 0; i < numPeers; i++ {
		peers[i] = PeerInfo{
			ID:      fmt.Sprintf("peer_%04d", i),
			Address: fmt.Sprintf("10.%d.%d.1:3000", (i/256)%256, i%256),
		}
	}

	for _, p := range peers {
		rt.AddPeer(p)
	}

	if rt.Size() != numPeers {
		t.Errorf("expected %d peers, got %d", numPeers, rt.Size())
	}

	// Find 20 closest to "peer_0500"
	closest := rt.FindClosest("peer_0500", 20)
	if len(closest) != 20 {
		t.Fatalf("expected 20 closest, got %d", len(closest))
	}

	// Verify results are sorted by distance
	for i := 1; i < len(closest); i++ {
		d1 := distance(closest[i-1].ID, "peer_0500")
		d2 := distance(closest[i].ID, "peer_0500")
		if d1 > d2 {
			t.Errorf("not sorted at index %d: dist(%s)=%d > dist(%s)=%d",
				i, closest[i-1].ID, d1, closest[i].ID, d2)
		}
	}

	// Verify first result is the closest possible
	minDist := math.MaxInt32
	var closestID string
	for _, p := range peers {
		d := distance(p.ID, "peer_0500")
		if d < minDist {
			minDist = d
			closestID = p.ID
		}
	}
	if closest[0].ID != closestID {
		t.Errorf("expected closest to be '%s' (dist=%d), got '%s' (dist=%d)",
			closestID, minDist, closest[0].ID, distance(closest[0].ID, "peer_0500"))
	}

	// Remove all peers and verify empty
	for _, p := range peers {
		rt.RemovePeer(p.ID)
	}
	if rt.Size() != 0 {
		t.Errorf("expected 0 peers after removal, got %d", rt.Size())
	}
}

func TestRoutingTable_BucketDistribution(t *testing.T) {
	rt := NewRoutingTable("self_0000")

	// Add peers that should distribute across buckets
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("peer_%03d_%s", i, string(rune('a'+i%26)))
		rt.AddPeer(PeerInfo{ID: id, Address: fmt.Sprintf("10.0.%d.1:3000", i%256)})
	}

	// Verify not all peers end up in the same bucket
	rt.mu.RLock()
	usedBuckets := 0
	for _, bucket := range rt.buckets {
		if len(bucket) > 0 {
			usedBuckets++
		}
	}
	rt.mu.RUnlock()

	if usedBuckets < 2 {
		t.Errorf("expected peers distributed across multiple buckets, only %d used", usedBuckets)
	}
}

func TestRoutingTable_SortedClosestIntegrity(t *testing.T) {
	rt := NewRoutingTable("self")

	for i := 0; i < 50; i++ {
		rt.AddPeer(PeerInfo{ID: fmt.Sprintf("node_%02d", i), Address: "10.0.0.1:3000"})
	}

	target := "node_25"
	closest := rt.FindClosest(target, 10)

	// Manually compute all distances and sort
	type pd struct {
		id   string
		dist int
	}
	var all []pd
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("node_%02d", i)
		all = append(all, pd{id: id, dist: distance(id, target)})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].dist < all[j].dist })

	// Compare top 10
	for i := 0; i < 10; i++ {
		if closest[i].ID != all[i].id {
			t.Errorf("position %d: expected '%s' (dist=%d), got '%s' (dist=%d)",
				i, all[i].id, all[i].dist, closest[i].ID, distance(closest[i].ID, target))
		}
	}
}
