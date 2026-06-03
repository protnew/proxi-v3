package crdt

import (
	"testing"
)

// ==================== LWWRegister Tests ====================

func TestLWWRegisterNew(t *testing.T) {
	r := NewLWWRegister("test-key")
	if r.Key() != "test-key" {
		t.Fatalf("expected key 'test-key', got '%s'", r.Key())
	}
	val, ts := r.Get()
	if len(val) != 0 {
		t.Fatalf("expected empty value, got %v", val)
	}
	if ts != 0 {
		t.Fatalf("expected timestamp 0, got %d", ts)
	}
}

func TestLWWRegisterSet(t *testing.T) {
	r := NewLWWRegister("key1")

	r.Set([]byte("hello"), 100)
	val, ts := r.Get()
	if string(val) != "hello" {
		t.Fatalf("expected 'hello', got '%s'", string(val))
	}
	if ts != 100 {
		t.Fatalf("expected timestamp 100, got %d", ts)
	}
}

func TestLWWRegisterSetOlderIgnored(t *testing.T) {
	r := NewLWWRegister("key1")

	r.Set([]byte("first"), 100)
	r.Set([]byte("second"), 50) // older, should be ignored

	val, ts := r.Get()
	if string(val) != "first" {
		t.Fatalf("expected 'first' (older write ignored), got '%s'", string(val))
	}
	if ts != 100 {
		t.Fatalf("expected timestamp 100, got %d", ts)
	}
}

func TestLWWRegisterSetNewer(t *testing.T) {
	r := NewLWWRegister("key1")

	r.Set([]byte("first"), 100)
	r.Set([]byte("second"), 200)

	val, ts := r.Get()
	if string(val) != "second" {
		t.Fatalf("expected 'second', got '%s'", string(val))
	}
	if ts != 200 {
		t.Fatalf("expected timestamp 200, got %d", ts)
	}
}

func TestLWWRegisterMerge(t *testing.T) {
	r1 := NewLWWRegister("key1")
	r2 := NewLWWRegister("key1")

	r1.Set([]byte("from-r1"), 100)
	r2.Set([]byte("from-r2"), 200)

	r1.Merge(r2)

	val, ts := r1.Get()
	if string(val) != "from-r2" {
		t.Fatalf("expected 'from-r2' after merge, got '%s'", string(val))
	}
	if ts != 200 {
		t.Fatalf("expected timestamp 200, got %d", ts)
	}
}

func TestLWWRegisterMergeOlderIgnored(t *testing.T) {
	r1 := NewLWWRegister("key1")
	r2 := NewLWWRegister("key1")

	r1.Set([]byte("newer"), 300)
	r2.Set([]byte("older"), 100)

	r1.Merge(r2)

	val, _ := r1.Get()
	if string(val) != "newer" {
		t.Fatalf("expected 'newer' after merge with older, got '%s'", string(val))
	}
}

func TestLWWRegisterGetReturnsCopy(t *testing.T) {
	r := NewLWWRegister("key1")
	r.Set([]byte("original"), 100)

	val, _ := r.Get()
	val[0] = 'X' // modify the copy

	original, _ := r.Get()
	if string(original) != "original" {
		t.Fatalf("Get should return a copy; original was modified")
	}
}

// ==================== GCounter Tests ====================

func TestGCounterNew(t *testing.T) {
	g := NewGCounter("node-1")
	if g.ID() != "node-1" {
		t.Fatalf("expected ID 'node-1', got '%s'", g.ID())
	}
	if g.Value() != 0 {
		t.Fatalf("expected value 0, got %d", g.Value())
	}
}

func TestGCounterIncrement(t *testing.T) {
	g := NewGCounter("node-1")

	g.Increment(5)
	if g.Value() != 5 {
		t.Fatalf("expected 5, got %d", g.Value())
	}

	g.Increment(3)
	if g.Value() != 8 {
		t.Fatalf("expected 8, got %d", g.Value())
	}
}

func TestGCounterMultipleNodes(t *testing.T) {
	g1 := NewGCounter("node-1")
	g2 := NewGCounter("node-2")

	g1.Increment(5)
	g2.Increment(3)

	g1.Merge(g2)

	if g1.Value() != 8 {
		t.Fatalf("expected 8 after merge, got %d", g1.Value())
	}

	// g2 should still be 3
	if g2.Value() != 3 {
		t.Fatalf("expected g2 to still be 3, got %d", g2.Value())
	}
}

func TestGCounterMergeIdempotent(t *testing.T) {
	g1 := NewGCounter("node-1")
	g2 := NewGCounter("node-2")

	g1.Increment(5)
	g2.Increment(3)

	g1.Merge(g2)
	g1.Merge(g2) // merge again, should be idempotent

	if g1.Value() != 8 {
		t.Fatalf("expected 8 after idempotent merge, got %d", g1.Value())
	}
}

func TestGCounterMergeTakesMax(t *testing.T) {
	g1 := NewGCounter("node-1")
	g2 := NewGCounter("node-1") // same node

	g1.Increment(5)
	g2.Increment(10)

	g1.Merge(g2)

	// Should take max for same node
	if g1.Value() != 10 {
		t.Fatalf("expected max=10 after merge, got %d", g1.Value())
	}
}

func TestGCounterCounts(t *testing.T) {
	g := NewGCounter("node-1")
	g.Increment(5)

	counts := g.Counts()
	if counts["node-1"] != 5 {
		t.Fatalf("expected node-1 count 5, got %d", counts["node-1"])
	}
}

// ==================== ORSet Tests ====================

func TestORSetNew(t *testing.T) {
	s := NewORSet("node-1")
	elems := s.Elements()
	if len(elems) != 0 {
		t.Fatalf("expected empty set, got %v", elems)
	}
}

func TestORSetAddContains(t *testing.T) {
	s := NewORSet("node-1")

	s.Add("apple")
	s.Add("banana")

	if !s.Contains("apple") {
		t.Fatal("expected to contain 'apple'")
	}
	if !s.Contains("banana") {
		t.Fatal("expected to contain 'banana'")
	}
	if s.Contains("cherry") {
		t.Fatal("expected NOT to contain 'cherry'")
	}
}

func TestORSetAddRemove(t *testing.T) {
	s := NewORSet("node-1")

	s.Add("apple")
	if !s.Contains("apple") {
		t.Fatal("expected to contain 'apple' after add")
	}

	s.Remove("apple")
	if s.Contains("apple") {
		t.Fatal("expected NOT to contain 'apple' after remove")
	}
}

func TestORSetRemoveNonExistent(t *testing.T) {
	s := NewORSet("node-1")
	s.Remove("ghost") // should be no-op
	if s.Contains("ghost") {
		t.Fatal("expected NOT to contain 'ghost'")
	}
}

func TestORSetAddRemoveReAdd(t *testing.T) {
	s := NewORSet("node-1")

	s.Add("apple")
	s.Remove("apple")
	s.Add("apple") // re-add should work

	if !s.Contains("apple") {
		t.Fatal("expected to contain 'apple' after re-add")
	}
}

func TestORSetMerge(t *testing.T) {
	s1 := NewORSet("node-1")
	s2 := NewORSet("node-2")

	s1.Add("apple")
	s1.Add("banana")
	s2.Add("cherry")
	s2.Add("date")

	s1.Merge(s2)

	for _, elem := range []string{"apple", "banana", "cherry", "date"} {
		if !s1.Contains(elem) {
			t.Fatalf("expected to contain '%s' after merge", elem)
		}
	}
}

func TestORSetMergeWithRemoves(t *testing.T) {
	s1 := NewORSet("node-1")
	s2 := NewORSet("node-2")

	// Both add "shared"
	s1.Add("shared")
	s2.Add("shared")

	// s2 removes it
	s2.Remove("shared")

	// Merge: remove should propagate
	s1.Merge(s2)

	if s1.Contains("shared") {
		t.Fatal("expected 'shared' to be removed after merge")
	}
}

func TestORSetMergeIdempotent(t *testing.T) {
	s1 := NewORSet("node-1")
	s2 := NewORSet("node-2")

	s1.Add("apple")
	s2.Add("banana")

	s1.Merge(s2)
	countBefore := len(s1.Elements())

	s1.Merge(s2)
	countAfter := len(s1.Elements())

	if countBefore != countAfter {
		t.Fatalf("merge should be idempotent: before=%d, after=%d", countBefore, countAfter)
	}
}

func TestORSetElements(t *testing.T) {
	s := NewORSet("node-1")
	s.Add("a")
	s.Add("b")
	s.Add("c")
	s.Remove("b")

	elems := s.Elements()
	if len(elems) != 2 {
		t.Fatalf("expected 2 elements, got %d: %v", len(elems), elems)
	}

	hasA, hasC := false, false
	for _, e := range elems {
		if e == "a" {
			hasA = true
		}
		if e == "c" {
			hasC = true
		}
	}
	if !hasA || !hasC {
		t.Fatalf("expected elements 'a' and 'c', got %v", elems)
	}
}
