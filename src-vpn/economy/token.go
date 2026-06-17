package economy

import (
	"crypto/rand"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TokenType categorizes the nature of a token operation.
type TokenType string

const (
	TokenReward  TokenType = "reward"
	TokenPayment TokenType = "payment"
	TokenStake   TokenType = "stake"
)

// Token represents a single token operation record.
type Token struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	Amount    int64     `json:"amount"`
	Type      TokenType `json:"type"`
	CreatedAt int64     `json:"created_at"` // unix timestamp
}

// globalTokenSeq is used to generate unique token IDs.
var globalTokenSeq uint64

func nextTokenID() string {
	seq := atomic.AddUint64(&globalTokenSeq, 1)
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("tok-%d-%x", seq, b)
}

// TokenLedger is a thread-safe in-memory ledger that tracks balances,
// stakes, and token operation history.
type TokenLedger struct {
	mu       sync.RWMutex
	balances map[string]int64 // npub → available balance
	stakes   map[string]int64 // nodeID → staked amount
	history  []Token
	total    int64 // total supply
}

// NewTokenLedger creates a new empty TokenLedger.
func NewTokenLedger() *TokenLedger {
	return &TokenLedger{
		balances: make(map[string]int64),
		stakes:   make(map[string]int64),
		history:  make([]Token, 0),
	}
}

// Mint creates new tokens and credits them to the given account.
func (l *TokenLedger) Mint(to string, amount int64, tType TokenType) error {
	if amount <= 0 {
		return fmt.Errorf("mint amount must be positive, got %d", amount)
	}
	if to == "" {
		return fmt.Errorf("recipient must not be empty")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.balances[to] += amount
	l.total += amount

	l.history = append(l.history, Token{
		ID:        nextTokenID(),
		Owner:     to,
		Amount:    amount,
		Type:      tType,
		CreatedAt: time.Now().Unix(),
	})

	return nil
}

// Transfer moves tokens from one account to another.
func (l *TokenLedger) Transfer(from, to string, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("transfer amount must be positive, got %d", amount)
	}
	if from == "" || to == "" {
		return fmt.Errorf("from and to must not be empty")
	}
	if from == to {
		return fmt.Errorf("cannot transfer to self")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	bal := l.balances[from]
	if bal < amount {
		return fmt.Errorf("insufficient balance: have %d, need %d", bal, amount)
	}

	l.balances[from] -= amount
	l.balances[to] += amount

	l.history = append(l.history, Token{
		ID:        nextTokenID(),
		Owner:     from,
		Amount:    -amount,
		Type:      TokenPayment,
		CreatedAt: time.Now().Unix(),
	})
	l.history = append(l.history, Token{
		ID:        nextTokenID(),
		Owner:     to,
		Amount:    amount,
		Type:      TokenPayment,
		CreatedAt: time.Now().Unix(),
	})

	return nil
}

// Burn destroys tokens from the given account.
func (l *TokenLedger) Burn(from string, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("burn amount must be positive, got %d", amount)
	}
	if from == "" {
		return fmt.Errorf("from must not be empty")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	bal := l.balances[from]
	if bal < amount {
		return fmt.Errorf("insufficient balance: have %d, need %d", bal, amount)
	}

	l.balances[from] -= amount
	l.total -= amount

	l.history = append(l.history, Token{
		ID:        nextTokenID(),
		Owner:     from,
		Amount:    -amount,
		Type:      TokenPayment,
		CreatedAt: time.Now().Unix(),
	})

	return nil
}

// Balance returns the available (non-staked) balance for an account.
func (l *TokenLedger) Balance(npub string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.balances[npub]
}

// Stake moves tokens from the available balance to the staked pool.
func (l *TokenLedger) Stake(nodeID string, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("stake amount must be positive, got %d", amount)
	}
	if nodeID == "" {
		return fmt.Errorf("nodeID must not be empty")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	bal := l.balances[nodeID]
	if bal < amount {
		return fmt.Errorf("insufficient balance to stake: have %d, need %d", bal, amount)
	}

	l.balances[nodeID] -= amount
	l.stakes[nodeID] += amount

	l.history = append(l.history, Token{
		ID:        nextTokenID(),
		Owner:     nodeID,
		Amount:    amount,
		Type:      TokenStake,
		CreatedAt: time.Now().Unix(),
	})

	return nil
}

// Slash reduces the staked amount for a node (penalty).
func (l *TokenLedger) Slash(nodeID string, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("slash amount must be positive, got %d", amount)
	}
	if nodeID == "" {
		return fmt.Errorf("nodeID must not be empty")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	staked := l.stakes[nodeID]
	if staked < amount {
		return fmt.Errorf("insufficient stake to slash: staked %d, requested %d", staked, amount)
	}

	l.stakes[nodeID] -= amount
	l.total -= amount

	l.history = append(l.history, Token{
		ID:        nextTokenID(),
		Owner:     nodeID,
		Amount:    -amount,
		Type:      TokenStake,
		CreatedAt: time.Now().Unix(),
	})

	return nil
}

// StakedBalance returns the staked amount for a node.
func (l *TokenLedger) StakedBalance(nodeID string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.stakes[nodeID]
}

// History returns all token operations for the given account.
func (l *TokenLedger) History(npub string) []Token {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []Token
	for _, tok := range l.history {
		if tok.Owner == npub {
			result = append(result, tok)
		}
	}
	if result == nil {
		result = []Token{}
	}
	return result
}

// TotalSupply returns the total amount of tokens currently in circulation
// (including staked tokens that haven't been slashed).
func (l *TokenLedger) TotalSupply() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.total
}

// ──────────────────────────────────────────────────────────────────────────────
// Integration hooks: economy rewards & slashing for proof-of-storage / bandwidth
// ──────────────────────────────────────────────────────────────────────────────
//
// These hooks tie the token ledger and reputation system to the proof loops.
// The storage/bandwidth schedulers call them via SchedulerCallbacks so that
// verified work is rewarded and missed work is slashed.

// Default reward and penalty amounts (in token units) when the caller does not
// specify an explicit amount.
const (
	DefaultStorageProofReward   int64 = 100
	DefaultBandwidthProofReward int64 = 50
	DefaultSlashAmount          int64 = 50
)

// RewardForStorageProof mints a reward to a node for a verified storage proof
// and raises its reputation. If amount <= 0 the default storage reward is used.
// Returns the amount minted.
func RewardForStorageProof(ledger *TokenLedger, rep *ReputationManager, nodeID string, amount int64) (int64, error) {
	if nodeID == "" {
		return 0, fmt.Errorf("nodeID must not be empty")
	}
	if amount <= 0 {
		amount = DefaultStorageProofReward
	}
	if err := ledger.Mint(nodeID, amount, TokenReward); err != nil {
		return 0, err
	}
	if rep != nil {
		rep.UpdateReputation(nodeID, true)
	}
	return amount, nil
}

// RewardForBandwidthProof mints a reward to a node for verified bandwidth
// (bytes served) and raises its reputation. If amount <= 0 the default
// bandwidth reward is used. Returns the amount minted.
func RewardForBandwidthProof(ledger *TokenLedger, rep *ReputationManager, nodeID string, amount int64) (int64, error) {
	if nodeID == "" {
		return 0, fmt.Errorf("nodeID must not be empty")
	}
	if amount <= 0 {
		amount = DefaultBandwidthProofReward
	}
	if err := ledger.Mint(nodeID, amount, TokenReward); err != nil {
		return 0, err
	}
	if rep != nil {
		rep.UpdateReputation(nodeID, true)
	}
	return amount, nil
}

// SlashForStorageFailure penalizes a node that failed or timed out a storage
// challenge. It lowers the node's reputation and slashes its stake (capped at
// the current stake). If the node has no stake, only reputation is penalized.
// If amount <= 0 the default slash amount is used. Returns the amount actually
// slashed (may be 0 if the node has no stake).
func SlashForStorageFailure(ledger *TokenLedger, rep *ReputationManager, nodeID string, amount int64) (int64, error) {
	if nodeID == "" {
		return 0, fmt.Errorf("nodeID must not be empty")
	}
	if amount <= 0 {
		amount = DefaultSlashAmount
	}
	if rep != nil {
		rep.UpdateReputation(nodeID, false)
	}
	if ledger == nil {
		return 0, nil
	}
	staked := ledger.StakedBalance(nodeID)
	if staked <= 0 {
		return 0, nil
	}
	actual := amount
	if actual > staked {
		actual = staked
	}
	if err := ledger.Slash(nodeID, actual); err != nil {
		return 0, err
	}
	return actual, nil
}
