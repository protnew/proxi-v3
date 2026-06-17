package economy

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

// fakeClock is a controllable clock for deterministic timeout tests.
type fakeClock struct {
	t time.Time
}

func (f *fakeClock) now() time.Time { return f.t }

func TestContentStorage_Fields(t *testing.T) {
	data := []byte("the actual content chunk bytes")
	hash := ChunkHash(data)
	cs := ContentStorage{
		ChunkHash: hash,
		NodeID:    "node-1",
		Timestamp: time.Now(),
		Challenge: GenerateChallenge(hash),
		Proof:     GenerateProofData(GenerateChallenge(hash), data),
		Verified:  true,
	}
	if cs.ChunkHash != hash {
		t.Errorf("chunk hash mismatch: got %s want %s", cs.ChunkHash, hash)
	}
	if cs.NodeID != "node-1" {
		t.Errorf("node id mismatch: %s", cs.NodeID)
	}
	if !cs.Verified {
		t.Error("should be verified")
	}
	if len(cs.Proof) == 0 {
		t.Error("proof should be non-empty")
	}
}

func TestChunkMap_AddGet(t *testing.T) {
	cm := NewChunkMap()
	data := []byte("hello mesh storage")
	h := cm.Add(data)
	got, ok := cm.GetChunk(h)
	if !ok {
		t.Fatal("chunk should be retrievable")
	}
	if !bytes.Equal(got, data) {
		t.Error("retrieved chunk differs from stored")
	}
	// chunk hash must be the canonical SHA-256 of the data
	if h != ChunkHash(data) {
		t.Error("Add should key by ChunkHash")
	}
	_, ok = cm.GetChunk("nonexistent")
	if ok {
		t.Error("nonexistent chunk should not be found")
	}
}

func TestScheduler_RegisterAndIssue(t *testing.T) {
	cm := NewChunkMap()
	data := []byte("content alpha")
	hash := cm.Add(data)

	sched := NewStorageChallengeScheduler(cm, time.Hour, time.Hour)
	sched.RegisterStorage("node-A", hash)
	sched.RegisterStorage("node-A", hash) // duplicate must be ignored

	nodes := sched.RegisteredNodes()
	if len(nodes) != 1 || nodes[0] != "node-A" {
		t.Fatalf("expected single node-A, got %v", nodes)
	}

	ch, ok := sched.IssueChallenge("node-A", hash)
	if !ok {
		t.Fatal("challenge should be issued for known chunk")
	}
	// chunk hash recorded on the challenge must match the real content hash
	if ch.ChunkHash != hash {
		t.Errorf("challenge chunk hash mismatch: got %s want %s", ch.ChunkHash, hash)
	}
	if sched.PendingCount() != 1 {
		t.Errorf("expected 1 pending, got %d", sched.PendingCount())
	}

	// Issuing for an unknown chunk must be refused.
	if _, ok := sched.IssueChallenge("node-A", "deadbeef"); ok {
		t.Error("must not issue challenge for unknown chunk")
	}
}

func TestScheduler_SubmitProof_Valid(t *testing.T) {
	cm := NewChunkMap()
	data := []byte("important content")
	hash := cm.Add(data)

	var successNode, successChunk string
	sched := NewStorageChallengeScheduler(cm, time.Hour, time.Hour)
	sched.SetCallbacks(SchedulerCallbacks{
		OnSuccess: func(n, c string) { successNode = n; successChunk = c },
	})
	sched.RegisterStorage("node-1", hash)

	ch, _ := sched.IssueChallenge("node-1", hash)
	proof := GenerateProofData(ch, data)

	passed, err := sched.SubmitProof("node-1", hash, proof)
	if err != nil {
		t.Fatalf("submit error: %v", err)
	}
	if !passed {
		t.Error("valid proof should pass")
	}
	if sched.PendingCount() != 0 {
		t.Error("pending should be cleared after valid proof")
	}
	if successNode != "node-1" || successChunk != hash {
		t.Errorf("OnSuccess not invoked correctly: %s/%s", successNode, successChunk)
	}

	results := sched.Results()
	if len(results) != 1 || !results[0].Passed {
		t.Errorf("expected 1 passed result, got %+v", results)
	}
}

func TestScheduler_SubmitProof_Invalid(t *testing.T) {
	cm := NewChunkMap()
	hash := cm.Add([]byte("real data"))

	var failureCalled bool
	sched := NewStorageChallengeScheduler(cm, time.Hour, time.Hour)
	sched.SetCallbacks(SchedulerCallbacks{OnFailure: func(n, c, reason string) { failureCalled = true }})
	sched.RegisterStorage("node-1", hash)
	sched.IssueChallenge("node-1", hash)

	passed, err := sched.SubmitProof("node-1", hash, []byte("totally wrong proof"))
	if err != nil {
		t.Fatalf("submit error: %v", err)
	}
	if passed {
		t.Error("invalid proof should not pass")
	}
	if !failureCalled {
		t.Error("OnFailure should be called for invalid proof")
	}
	results := sched.Results()
	if len(results) != 1 || results[0].Passed {
		t.Error("expected a failed result")
	}
}

func TestScheduler_SubmitProof_NoPending(t *testing.T) {
	cm := NewChunkMap()
	cm.Add([]byte("data"))
	sched := NewStorageChallengeScheduler(cm, time.Hour, time.Hour)
	if _, err := sched.SubmitProof("ghost", "x", []byte("p")); err == nil {
		t.Error("expected error when no pending challenge")
	}
}

func TestScheduler_Timeout_TriggersPenalty(t *testing.T) {
	cm := NewChunkMap()
	hash := cm.Add([]byte("chunk that will time out"))

	clock := &fakeClock{t: time.Unix(1000, 0)}
	var timeoutNode, timeoutChunk, timeoutReason string
	sched := NewStorageChallengeScheduler(cm, time.Hour, 5*time.Second)
	sched.SetClock(clock.now)
	sched.SetCallbacks(SchedulerCallbacks{
		OnFailure: func(n, c, reason string) {
			timeoutNode = n
			timeoutChunk = c
			timeoutReason = reason
		},
	})
	sched.RegisterStorage("laggy-node", hash)

	if _, ok := sched.IssueChallenge("laggy-node", hash); !ok {
		t.Fatal("challenge should be issued")
	}
	if sched.PendingCount() != 1 {
		t.Fatalf("expected 1 pending, got %d", sched.PendingCount())
	}

	// No timeout yet (issued just now on the fake clock).
	if n := sched.CheckTimeouts(); n != 0 {
		t.Errorf("expected 0 timeouts before deadline, got %d", n)
	}

	// Advance clock past the timeout window.
	clock.t = clock.t.Add(6 * time.Second)
	n := sched.CheckTimeouts()
	if n != 1 {
		t.Fatalf("expected 1 timeout, got %d", n)
	}
	if sched.PendingCount() != 0 {
		t.Error("pending should be cleared after timeout")
	}
	if timeoutNode != "laggy-node" {
		t.Errorf("OnFailure node mismatch: %s", timeoutNode)
	}
	if timeoutChunk != hash {
		t.Errorf("OnFailure chunk mismatch: %s", timeoutChunk)
	}
	if timeoutReason != "timeout" {
		t.Errorf("OnFailure reason should be timeout, got %s", timeoutReason)
	}

	results := sched.Results()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].TimedOut || results[0].Passed {
		t.Errorf("expected timed-out failed result, got %+v", results[0])
	}
}

func TestScheduler_ChallengeLogging(t *testing.T) {
	cm := NewChunkMap()
	hash := cm.Add([]byte("logged chunk"))
	var buf bytes.Buffer
	sched := NewStorageChallengeScheduler(cm, time.Hour, time.Hour)
	sched.SetLogger(log.New(&buf, "test ", log.LstdFlags))
	sched.RegisterStorage("node-X", hash)
	sched.IssueChallenge("node-X", hash)

	out := buf.String()
	if !strings.Contains(out, "challenge issued") {
		t.Errorf("log should mention challenge issued, got: %s", out)
	}
	if !strings.Contains(out, "node-X") {
		t.Errorf("log should mention node, got: %s", out)
	}
}

func TestScheduler_RunOnce_FullFlow(t *testing.T) {
	cm := NewChunkMap()
	d1 := []byte("chunk one")
	d2 := []byte("chunk two")
	h1 := cm.Add(d1)
	h2 := cm.Add(d2)

	sched := NewStorageChallengeScheduler(cm, time.Hour, time.Hour)
	sched.RegisterStorage("node-1", h1)
	sched.RegisterStorage("node-2", h2)

	// RunOnce issues a challenge to every registered pair.
	sched.RunOnce()
	if sched.PendingCount() != 2 {
		t.Fatalf("expected 2 pending after RunOnce, got %d", sched.PendingCount())
	}

	// Fetch each pending challenge and answer it correctly.
	answer := func(node string, hash string, data []byte) {
		ch, ok := sched.PendingChallenge(node, hash)
		if !ok {
			t.Fatalf("no pending challenge for %s/%s", node, hash)
		}
		if ch.ChunkHash != hash {
			t.Errorf("pending challenge chunk hash mismatch: got %s want %s", ch.ChunkHash, hash)
		}
		passed, err := sched.SubmitProof(node, hash, GenerateProofData(ch, data))
		if err != nil || !passed {
			t.Fatalf("proof for %s should pass: %v passed=%v", node, err, passed)
		}
	}
	answer("node-1", h1, d1)
	answer("node-2", h2, d2)

	if sched.PendingCount() != 0 {
		t.Errorf("expected 0 pending after answering, got %d", sched.PendingCount())
	}
	results := sched.Results()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("result for %s should be passed", r.NodeID)
		}
	}
}

func TestScheduler_StartStop(t *testing.T) {
	cm := NewChunkMap()
	hash := cm.Add([]byte("async chunk"))
	// timeout (10ms) < interval (30ms) so an unanswered challenge ages out
	// before the next round re-issues it.
	sched := NewStorageChallengeScheduler(cm, 30*time.Millisecond, 10*time.Millisecond)
	sched.RegisterStorage("node-async", hash)

	sched.Start()
	// A few rounds should happen, leaving timed-out results.
	time.Sleep(200 * time.Millisecond)
	sched.Stop()

	// Calling Stop again must be safe.
	sched.Stop()

	results := sched.Results()
	if len(results) == 0 {
		t.Fatal("expected scheduler to produce results while running")
	}
	for _, r := range results {
		if !r.TimedOut {
			t.Errorf("expected timed-out result, got %+v", r)
		}
	}
}

func TestScheduler_Unregister(t *testing.T) {
	cm := NewChunkMap()
	h := cm.Add([]byte("x"))
	sched := NewStorageChallengeScheduler(cm, time.Hour, time.Hour)
	sched.RegisterStorage("n1", h)
	sched.Unregister("n1")
	sched.RunOnce()
	if sched.PendingCount() != 0 {
		t.Error("unregistered node should not be challenged")
	}
}

func TestScheduler_ResultsBounded(t *testing.T) {
	cm := NewChunkMap()
	hash := cm.Add([]byte("data"))
	s := NewStorageChallengeScheduler(cm, time.Hour, time.Hour)
	s.resultsLimit = 3
	for i := 0; i < 10; i++ {
		s.pushResult(ProofResult{NodeID: "n", ChunkHash: hash, Passed: true})
	}
	if len(s.Results()) != 3 {
		t.Errorf("expected 3 trimmed results, got %d", len(s.Results()))
	}
}
