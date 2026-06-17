package economy

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// TransferRecord records a bandwidth transfer between two nodes.
type TransferRecord struct {
	SenderID   string    // node that sent data
	ReceiverID string    // node that received data
	BytesSent  int64     // number of bytes transferred
	Timestamp  time.Time // when the transfer occurred
	Signature  []byte    // signature over the record fields
}

// transferSignPayload creates the byte payload to sign/verify.
func transferSignPayload(tr *TransferRecord) []byte {
	h := sha256.New()
	h.Write([]byte(tr.SenderID))
	h.Write([]byte(tr.ReceiverID))
	// encode BytesSent as hex to avoid ambiguity
	h.Write([]byte(hex.EncodeToString([]byte{
		byte(tr.BytesSent >> 56),
		byte(tr.BytesSent >> 48),
		byte(tr.BytesSent >> 40),
		byte(tr.BytesSent >> 32),
		byte(tr.BytesSent >> 24),
		byte(tr.BytesSent >> 16),
		byte(tr.BytesSent >> 8),
		byte(tr.BytesSent),
	})))
	h.Write([]byte(tr.Timestamp.Format(time.RFC3339Nano)))
	return h.Sum(nil)
}

// RecordTransfer creates a TransferRecord signed by the sender's private key.
func RecordTransfer(senderID, receiverID string, bytesSent int64, senderKey *ecdsa.PrivateKey) (*TransferRecord, error) {
	tr := &TransferRecord{
		SenderID:   senderID,
		ReceiverID: receiverID,
		BytesSent:  bytesSent,
		Timestamp:  time.Now(),
	}

	payload := transferSignPayload(tr)
	sig, err := ecdsa.SignASN1(rand.Reader, senderKey, payload)
	if err != nil {
		return nil, err
	}
	tr.Signature = sig
	return tr, nil
}

// VerifyTransfer verifies the signature on a TransferRecord using the sender's public key.
func VerifyTransfer(tr *TransferRecord, senderPubKey *ecdsa.PublicKey) bool {
	payload := transferSignPayload(tr)
	return ecdsa.VerifyASN1(senderPubKey, payload, tr.Signature)
}

// ──────────────────────────────────────────────────────────────────────────────
// Mesh transfer tracking (proof-of-bandwidth)
// ──────────────────────────────────────────────────────────────────────────────

// GamingReport describes a detected discrepancy between the bytes a sender
// reports sending and the bytes the receiver reports receiving.
type GamingReport struct {
	Sender         string
	Receiver       string
	SenderReported int64 // bytes the sender claims to have sent
	ReceiverReported int64 // bytes the receiver claims to have received
}

// TransferTracker aggregates bandwidth counters reported independently by each
// peer in the mesh. It implements peer verification and anti-gaming: a transfer
// is only "confirmed" (and thus reward-eligible) when the sender's reported
// bytes-sent equals the receiver's reported bytes-received.
//
// All counters are unidirectional keyed as "from -> to".
type TransferTracker struct {
	mu        sync.Mutex
	sent      map[string]map[string]int64 // sent[a][b] = a reports sending to b
	recv      map[string]map[string]int64 // recv[b][a] = b reports receiving from a
	confirmed map[string]map[string]int64 // confirmed[a][b] = agreed bytes a->b
	flagged   map[string]map[string]bool  // flagged[a][b] = gaming detected a->b
}

// NewTransferTracker creates an empty tracker.
func NewTransferTracker() *TransferTracker {
	return &TransferTracker{
		sent:      make(map[string]map[string]int64),
		recv:      make(map[string]map[string]int64),
		confirmed: make(map[string]map[string]int64),
		flagged:   make(map[string]map[string]bool),
	}
}

func nestedGet(m map[string]map[string]int64, a, b string) int64 {
	if inner, ok := m[a]; ok {
		return inner[b]
	}
	return 0
}

func nestedSetInt64(m map[string]map[string]int64, a, b string, v int64) {
	inner, ok := m[a]
	if !ok {
		inner = make(map[string]int64)
		m[a] = inner
	}
	inner[b] = v
}

func nestedSetBool(m map[string]map[string]bool, a, b string, v bool) {
	inner, ok := m[a]
	if !ok {
		inner = make(map[string]bool)
		m[a] = inner
	}
	inner[b] = v
}

// RecordSent records that the sender transferred bytes to the receiver.
// Negative values are clamped to zero.
func (t *TransferTracker) RecordSent(sender, receiver string, bytes int64) {
	if bytes < 0 {
		bytes = 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	nestedSetInt64(t.sent, sender, receiver, nestedGet(t.sent, sender, receiver)+bytes)
}

// RecordReceived records that the receiver received bytes from the sender.
// Negative values are clamped to zero.
func (t *TransferTracker) RecordReceived(receiver, sender string, bytes int64) {
	if bytes < 0 {
		bytes = 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	nestedSetInt64(t.recv, receiver, sender, nestedGet(t.recv, receiver, sender)+bytes)
}

// VerifyPair checks the anti-gaming invariant for a sender->receiver transfer:
// the sender's reported bytes-sent must equal the receiver's reported
// bytes-received. On a match the volume is "confirmed" (reward-eligible). On a
// mismatch the pair is flagged for gaming and an error is returned.
//
// Returns (confirmed bytes, error).
func (t *TransferTracker) VerifyPair(sender, receiver string) (int64, error) {
	t.mu.Lock()
	s := nestedGet(t.sent, sender, receiver)
	r := nestedGet(t.recv, receiver, sender)
	if s != r {
		nestedSetBool(t.flagged, sender, receiver, true)
		t.mu.Unlock()
		return 0, fmt.Errorf("anti-gaming: sender %s reports %d bytes, receiver %s reports %d bytes", sender, s, receiver, r)
	}
	// clear any stale flag if they now agree
	nestedSetBool(t.flagged, sender, receiver, false)
	nestedSetInt64(t.confirmed, sender, receiver, s)
	t.mu.Unlock()
	return s, nil
}

// Confirmed returns the verified bytes transferred from sender to receiver
// (only non-zero after a successful VerifyPair).
func (t *TransferTracker) Confirmed(sender, receiver string) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return nestedGet(t.confirmed, sender, receiver)
}

// IsFlagged reports whether a sender->receiver pair has been flagged for gaming.
func (t *TransferTracker) IsFlagged(sender, receiver string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if inner, ok := t.flagged[sender]; ok {
		return inner[receiver]
	}
	return false
}

// TotalConfirmed returns the total verified bytes a node was involved in, both
// as sender and receiver.
func (t *TransferTracker) TotalConfirmed(nodeID string) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	var total int64
	for _, v := range t.confirmed[nodeID] {
		total += v
	}
	for _, inner := range t.confirmed {
		total += inner[nodeID]
	}
	return total
}

// AntiGamingScan walks every sender->receiver pair and returns a report for each
// pair whose reported sent/received volumes disagree.
func (t *TransferTracker) AntiGamingScan() []GamingReport {
	t.mu.Lock()
	defer t.mu.Unlock()
	var reports []GamingReport
	for sender, inner := range t.sent {
		for receiver, s := range inner {
			r := nestedGet(t.recv, receiver, sender)
			if s != r {
				nestedSetBool(t.flagged, sender, receiver, true)
				reports = append(reports, GamingReport{
					Sender:           sender,
					Receiver:         receiver,
					SenderReported:   s,
					ReceiverReported: r,
				})
			}
		}
	}
	return reports
}

// Reset clears all counters and flags.
func (t *TransferTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sent = make(map[string]map[string]int64)
	t.recv = make(map[string]map[string]int64)
	t.confirmed = make(map[string]map[string]int64)
	t.flagged = make(map[string]map[string]bool)
}
