package economy

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

// Challenge represents a proof-of-storage challenge sent to a node.
type Challenge struct {
	ChunkHash string    // hex-encoded SHA-256 of the original chunk
	Nonce     string    // random nonce for this challenge
	Timestamp time.Time // when the challenge was generated
}

// GenerateChallenge creates a new storage challenge for the given chunk hash.
func GenerateChallenge(chunkHash string) Challenge {
	nonceBytes := make([]byte, 32)
	_, _ = rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)

	return Challenge{
		ChunkHash: chunkHash,
		Nonce:     nonce,
		Timestamp: time.Now(),
	}
}

// GenerateProofData creates a proof response for a given challenge and original data.
// The proof is: SHA256(originalData || nonce || chunkHash).
// The node must possess originalData to compute this.
func GenerateProofData(challenge Challenge, originalData []byte) []byte {
	h := sha256.New()
	h.Write(originalData)
	h.Write([]byte(challenge.Nonce))
	h.Write([]byte(challenge.ChunkHash))
	return h.Sum(nil)
}

// VerifyProof checks that a node's proof is valid for the given challenge and original data.
// It re-computes the expected hash and compares against proofData.
func VerifyProof(challenge Challenge, proofData []byte, originalData []byte) bool {
	expected := GenerateProofData(challenge, originalData)
	if len(proofData) != len(expected) {
		return false
	}
	for i := range expected {
		if proofData[i] != expected[i] {
			return false
		}
	}
	return true
}

// ChunkHash computes the hex-encoded SHA-256 of data, used as a chunk identifier.
func ChunkHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ──────────────────────────────────────────────────────────────────────────────
// Content storage tracking
// ──────────────────────────────────────────────────────────────────────────────

// ContentStorage is a record that a node is storing a specific content chunk.
// It mirrors a table with columns: chunk_hash, node_id, timestamp, challenge, proof.
type ContentStorage struct {
	ChunkHash string    `json:"chunk_hash"`
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`
	Challenge Challenge `json:"challenge"`
	Proof     []byte    `json:"proof"`
	Verified  bool      `json:"verified"`
}

// ChunkRetriever returns the original chunk bytes for a chunk hash. The
// scheduler uses it to verify that a node actually stores the real content.
type ChunkRetriever interface {
	GetChunk(chunkHash string) ([]byte, bool)
}

// ChunkMap is a simple in-memory ChunkRetriever backed by a map.
type ChunkMap struct {
	mu     sync.RWMutex
	chunks map[string][]byte
}

// NewChunkMap creates an empty ChunkMap.
func NewChunkMap() *ChunkMap {
	return &ChunkMap{chunks: make(map[string][]byte)}
}

// Add stores a chunk, keying it by its ChunkHash, and returns that hash.
func (c *ChunkMap) Add(data []byte) string {
	h := ChunkHash(data)
	cp := make([]byte, len(data))
	copy(cp, data)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.chunks[h] = cp
	return h
}

// GetChunk implements ChunkRetriever.
func (c *ChunkMap) GetChunk(chunkHash string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, ok := c.chunks[chunkHash]
	return data, ok
}

// ──────────────────────────────────────────────────────────────────────────────
// Challenge scheduler
// ──────────────────────────────────────────────────────────────────────────────

// ProofResult records the outcome of a single storage challenge.
type ProofResult struct {
	NodeID    string    `json:"node_id"`
	ChunkHash string    `json:"chunk_hash"`
	Passed    bool      `json:"passed"`
	TimedOut  bool      `json:"timed_out"`
	At        time.Time `json:"at"`
	Reason    string    `json:"reason,omitempty"`
}

// SchedulerCallbacks lets the caller react to challenge outcomes (e.g. reward or
// slash). Any field may be nil; nil callbacks are no-ops.
type SchedulerCallbacks struct {
	OnSuccess func(nodeID, chunkHash string)
	OnFailure func(nodeID, chunkHash string, reason string)
}

type pendingChallenge struct {
	challenge Challenge
	issuedAt  time.Time
}

// StorageChallengeScheduler periodically issues proof-of-storage challenges to
// nodes, verifies their responses against the real content, and penalizes nodes
// that fail to respond before the timeout.
//
// It is safe for concurrent use. The scheduler can be driven manually via
// RunOnce()/CheckTimeouts() for deterministic testing, or run automatically via
// Start()/Stop().
type StorageChallengeScheduler struct {
	mu           sync.Mutex
	interval     time.Duration
	timeout      time.Duration
	retriever    ChunkRetriever
	storage      map[string][]string          // nodeID -> chunk hashes it stores
	pending      map[string]pendingChallenge  // key "nodeID|chunkHash" -> pending
	results      []ProofResult
	resultsLimit int

	log       *log.Logger
	callbacks SchedulerCallbacks
	now       func() time.Time // injectable clock

	stopCh  chan struct{}
	stopped bool
}

// NewStorageChallengeScheduler builds a scheduler that issues a challenge round
// every interval and times out unanswered challenges after timeout.
func NewStorageChallengeScheduler(retriever ChunkRetriever, interval, timeout time.Duration) *StorageChallengeScheduler {
	return &StorageChallengeScheduler{
		interval:     interval,
		timeout:      timeout,
		retriever:    retriever,
		storage:      make(map[string][]string),
		pending:      make(map[string]pendingChallenge),
		results:      make([]ProofResult, 0),
		resultsLimit: 1024,
		log:          log.New(io.Discard, "storage-proof ", log.LstdFlags),
		now:          time.Now,
	}
}

// SetLogger overrides the default (discard) logger. Pass nil to silence.
func (s *StorageChallengeScheduler) SetLogger(l *log.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l == nil {
		s.log = log.New(io.Discard, "storage-proof ", log.LstdFlags)
	} else {
		s.log = l
	}
}

// SetClock overrides the clock used for timeout calculations (mainly for tests).
func (s *StorageChallengeScheduler) SetClock(f func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f != nil {
		s.now = f
	}
}

// SetCallbacks installs success/failure callbacks.
func (s *StorageChallengeScheduler) SetCallbacks(cb SchedulerCallbacks) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callbacks = cb
}

// RegisterStorage records that a node is storing a chunk.
func (s *StorageChallengeScheduler) RegisterStorage(nodeID, chunkHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.storage[nodeID] {
		if c == chunkHash {
			return // already registered
		}
	}
	s.storage[nodeID] = append(s.storage[nodeID], chunkHash)
	s.log.Printf("registered node=%s chunk=%s", nodeID, chunkHash)
}

// Unregister removes all chunk registrations for a node.
func (s *StorageChallengeScheduler) Unregister(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.storage, nodeID)
}

// IssueChallenge creates and records a pending challenge for a node/chunk pair.
// It returns the challenge and true if a challenge is active for the pair, or
// false if the chunk is unknown (so verification would be impossible).
//
// A challenge for a pair stays outstanding until answered (SubmitProof) or timed
// out (CheckTimeouts); calling IssueChallenge again while one is pending simply
// returns the existing challenge without resetting its issue time. This keeps
// the timeout window meaningful across periodic RunOnce rounds.
func (s *StorageChallengeScheduler) IssueChallenge(nodeID, chunkHash string) (Challenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.retriever.GetChunk(chunkHash); !ok {
		s.log.Printf("skip challenge node=%s chunk=%s: chunk unknown", nodeID, chunkHash)
		return Challenge{}, false
	}
	key := nodeID + "|" + chunkHash
	if pc, ok := s.pending[key]; ok {
		// Already an outstanding challenge: reuse it, do not reset the clock.
		return pc.challenge, true
	}
	ch := GenerateChallenge(chunkHash)
	s.pending[key] = pendingChallenge{challenge: ch, issuedAt: s.now()}
	s.log.Printf("challenge issued node=%s chunk=%s nonce=%s", nodeID, chunkHash, ch.Nonce[:8])
	return ch, true
}

// SubmitProof resolves a pending challenge with a proof from the node.
// Returns (passed, error). A valid proof clears the pending entry and records a
// success; an invalid proof records a failure. Missing pending challenge or
// missing chunk data yield an error.
func (s *StorageChallengeScheduler) SubmitProof(nodeID, chunkHash string, proof []byte) (bool, error) {
	s.mu.Lock()
	key := nodeID + "|" + chunkHash
	pc, ok := s.pending[key]
	if !ok {
		s.mu.Unlock()
		return false, fmt.Errorf("no pending challenge for node=%s chunk=%s", nodeID, chunkHash)
	}
	data, hasData := s.retriever.GetChunk(chunkHash)
	if !hasData {
		delete(s.pending, key)
		s.mu.Unlock()
		return false, fmt.Errorf("chunk data unavailable for chunk=%s", chunkHash)
	}
	passed := VerifyProof(pc.challenge, proof, data)
	delete(s.pending, key)
	cb := s.callbacks
	logger := s.log
	now := s.now()
	if passed {
		s.pushResult(ProofResult{NodeID: nodeID, ChunkHash: chunkHash, Passed: true, At: now})
		logger.Printf("proof VERIFIED node=%s chunk=%s", nodeID, chunkHash)
	} else {
		s.pushResult(ProofResult{NodeID: nodeID, ChunkHash: chunkHash, Passed: false, At: now, Reason: "invalid proof"})
		logger.Printf("proof FAILED node=%s chunk=%s", nodeID, chunkHash)
	}
	s.mu.Unlock()

	if passed && cb.OnSuccess != nil {
		cb.OnSuccess(nodeID, chunkHash)
	} else if !passed && cb.OnFailure != nil {
		cb.OnFailure(nodeID, chunkHash, "invalid proof")
	}
	return passed, nil
}

// CheckTimeouts scans pending challenges and fails any whose age exceeds the
// timeout. Timed-out challenges invoke OnFailure with reason "timeout".
// Returns the number of challenges that timed out.
func (s *StorageChallengeScheduler) CheckTimeouts() int {
	s.mu.Lock()
	now := s.now()
	var timedOut []ProofResult
	for key, pc := range s.pending {
		if now.Sub(pc.issuedAt) >= s.timeout {
			parts := splitKey(key)
			timedOut = append(timedOut, ProofResult{
				NodeID:    parts[0],
				ChunkHash: parts[1],
				Passed:    false,
				TimedOut:  true,
				At:        now,
				Reason:    "timeout",
			})
			delete(s.pending, key)
		}
	}
	for _, r := range timedOut {
		s.pushResult(r)
	}
	cb := s.callbacks
	logger := s.log
	s.mu.Unlock()

	for _, r := range timedOut {
		logger.Printf("challenge TIMEOUT node=%s chunk=%s", r.NodeID, r.ChunkHash)
		if cb.OnFailure != nil {
			cb.OnFailure(r.NodeID, r.ChunkHash, "timeout")
		}
	}
	return len(timedOut)
}

// RunOnce issues a fresh challenge to every registered (node, chunk) pair and
// then checks for timeouts. It is the single round executed periodically by
// Start(), and can be called directly in tests.
func (s *StorageChallengeScheduler) RunOnce() {
	s.mu.Lock()
	type reg struct{ node, chunk string }
	regs := make([]reg, 0)
	for node, chunks := range s.storage {
		for _, c := range chunks {
			regs = append(regs, reg{node: node, chunk: c})
		}
	}
	s.mu.Unlock()

	for _, r := range regs {
		s.IssueChallenge(r.node, r.chunk)
	}
	s.CheckTimeouts()
}

// Start launches the periodic scheduler in a background goroutine. Call Stop()
// to terminate it. Calling Start more than once is a no-op.
func (s *StorageChallengeScheduler) Start() {
	s.mu.Lock()
	if s.stopped || s.stopCh != nil {
		s.mu.Unlock()
		return
	}
	s.stopCh = make(chan struct{})
	stopCh := s.stopCh
	interval := s.interval
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				s.RunOnce()
			}
		}
	}()
}

// Stop terminates the background scheduler. It is safe to call multiple times.
func (s *StorageChallengeScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopCh != nil && !s.stopped {
		close(s.stopCh)
		s.stopped = true
	}
}

// Results returns a copy of the recorded proof results.
func (s *StorageChallengeScheduler) Results() []ProofResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ProofResult, len(s.results))
	copy(out, s.results)
	return out
}

// PendingCount returns the number of unanswered challenges.
func (s *StorageChallengeScheduler) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// PendingChallenge returns the currently outstanding challenge for a node/chunk
// pair, if any. Useful for callers (and tests) that need the challenge nonce to
// compute a proof after a RunOnce round.
func (s *StorageChallengeScheduler) PendingChallenge(nodeID, chunkHash string) (Challenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pc, ok := s.pending[nodeID+"|"+chunkHash]
	return pc.challenge, ok
}

// RegisteredNodes returns the node IDs currently registered with the scheduler.
func (s *StorageChallengeScheduler) RegisteredNodes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.storage))
	for n := range s.storage {
		out = append(out, n)
	}
	return out
}

// pushResult appends a result, keeping only the most recent resultsLimit.
func (s *StorageChallengeScheduler) pushResult(r ProofResult) {
	s.results = append(s.results, r)
	if len(s.results) > s.resultsLimit {
		s.results = s.results[len(s.results)-s.resultsLimit:]
	}
}

// splitKey splits a "node|chunk" key back into its two parts.
func splitKey(key string) [2]string {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return [2]string{key[:i], key[i+1:]}
		}
	}
	return [2]string{key, ""}
}
