package payment

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// LightningClient interacts with LND REST API for Lightning Network payments.
type LightningClient struct {
	LNDHost  string
	Macaroon string
	TLSCert  string
	client   *http.Client
}

// NewLightningClient creates a new LND client.
func NewLightningClient(host, macaroon, tlsCert string) *LightningClient {
	return &LightningClient{
		LNDHost:  host,
		Macaroon: macaroon,
		TLSCert:  tlsCert,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// InvoiceResponse represents an LND invoice.
type InvoiceResponse struct {
	RHash          string `json:"r_hash"`
	PaymentRequest string `json:"payment_request"`
	AddIndex       string `json:"add_index"`
}

// CreateInvoice creates a Lightning invoice.
func (l *LightningClient) CreateInvoice(amountSats int64, description string) (paymentHash, paymentRequest string, err error) {
	body, _ := json.Marshal(map[string]interface{}{
		"value":        amountSats,
		"memo":         description,
		"expiry":       3600,
		"private":      true,
	})
	req, err := http.NewRequest("POST", l.LNDHost+"/v1/invoices", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Grpc-Metadata-macaroon", l.Macaroon)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("lnd request: %w", err)
	}
	defer resp.Body.Close()

	var inv InvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		return "", "", fmt.Errorf("decode invoice: %w", err)
	}
	return inv.RHash, inv.PaymentRequest, nil
}

// PayInvoice pays a Lightning invoice.
func (l *LightningClient) PayInvoice(paymentRequest string) (paymentHash string, err error) {
	body, _ := json.Marshal(map[string]string{"payment_request": paymentRequest})
	req, _ := http.NewRequest("POST", l.LNDHost+"/v2/router/send", bytes.NewReader(body))
	req.Header.Set("Grpc-Metadata-macaroon", l.Macaroon)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("lnd pay: %w", err)
	}
	defer resp.Body.Close()
	return paymentRequest[:32], nil
}

// CheckPayment checks if an invoice has been paid.
func (l *LightningClient) CheckPayment(paymentHash string) (paid bool, err error) {
	req, _ := http.NewRequest("GET", l.LNDHost+"/v1/invoice/"+paymentHash, nil)
	req.Header.Set("Grpc-Metadata-macaroon", l.Macaroon)
	resp, err := l.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	settled, _ := result["settled"].(bool)
	return settled, nil
}

// IsAvailable checks if LND node is reachable.
func (l *LightningClient) IsAvailable() bool {
	req, _ := http.NewRequest("GET", l.LNDHost+"/v1/getinfo", nil)
	req.Header.Set("Grpc-Metadata-macaroon", l.Macaroon)
	resp, err := l.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// ──────────────────────────────────────────────────────────────────────────────
// LND health check
// ──────────────────────────────────────────────────────────────────────────────

// LNDHealth is the result of an LND health probe.
type LNDHealth struct {
	Online  bool          `json:"online"`
	Latency time.Duration `json:"latency_ms"`
	Version string        `json:"version"`
	Alias   string        `json:"alias"`
	Error   string        `json:"error,omitempty"`
}

// GetInfoResponse mirrors the subset of LND's /v1/getinfo response we need.
type GetInfoResponse struct {
	Version string `json:"version"`
	Alias   string `json:"alias"`
	Synced  bool   `json:"synced_to_chain"`
}

// HealthCheck probes the LND node and returns reachability, latency and
// metadata. Unlike IsAvailable it also reports why the probe failed.
func (l *LightningClient) HealthCheck() LNDHealth {
	start := time.Now()
	req, err := http.NewRequest("GET", l.LNDHost+"/v1/getinfo", nil)
	if err != nil {
		return LNDHealth{Online: false, Error: err.Error()}
	}
	req.Header.Set("Grpc-Metadata-macaroon", l.Macaroon)
	resp, err := l.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return LNDHealth{Online: false, Latency: latency, Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return LNDHealth{Online: false, Latency: latency, Error: fmt.Sprintf("status %d", resp.StatusCode)}
	}
	var info GetInfoResponse
	_ = json.NewDecoder(resp.Body).Decode(&info) // best-effort
	return LNDHealth{Online: true, Latency: latency, Version: info.Version, Alias: info.Alias}
}

// ──────────────────────────────────────────────────────────────────────────────
// Invoice lifecycle management
// ──────────────────────────────────────────────────────────────────────────────

// InvoiceState tracks an invoice through its lifecycle.
type InvoiceState string

const (
	InvoiceStateOpen     InvoiceState = "open"
	InvoiceStateSettled  InvoiceState = "settled"
	InvoiceStateArchived InvoiceState = "archived"
	InvoiceStateExpired  InvoiceState = "expired"
	InvoiceStateCanceled InvoiceState = "canceled"
)

// TrackedInvoice is a locally tracked invoice with explicit lifecycle state.
type TrackedInvoice struct {
	RHash          string       `json:"r_hash"`
	PaymentRequest string       `json:"payment_request"`
	AmountSats     int64        `json:"amount_sats"`
	Memo           string       `json:"memo"`
	State          InvoiceState `json:"state"`
	CreatedAt      int64        `json:"created_at"`
	SettledAt      int64        `json:"settled_at"`
	ArchivedAt     int64        `json:"archived_at"`
}

// newRHash generates a random hex payment hash (for offline/test invoices).
func newRHash() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// InvoiceManager tracks invoices locally and drives their lifecycle. It can
// operate against a real LND (via LightningClient) or fully offline for tests.
type InvoiceManager struct {
	mu       sync.Mutex
	client   *LightningClient
	invoices map[string]*TrackedInvoice // rHash -> invoice
}

// NewInvoiceManager creates an InvoiceManager. client may be nil for offline use.
func NewInvoiceManager(client *LightningClient) *InvoiceManager {
	return &InvoiceManager{
		client:   client,
		invoices: make(map[string]*TrackedInvoice),
	}
}

// CreateLocalInvoice creates an open invoice without contacting LND (offline/test).
func (m *InvoiceManager) CreateLocalInvoice(amountSats int64, memo string) *TrackedInvoice {
	inv := &TrackedInvoice{
		RHash:          newRHash(),
		PaymentRequest: fmt.Sprintf("lnbc%doffline", amountSats),
		AmountSats:     amountSats,
		Memo:           memo,
		State:          InvoiceStateOpen,
		CreatedAt:      time.Now().Unix(),
	}
	m.mu.Lock()
	m.invoices[inv.RHash] = inv
	m.mu.Unlock()
	return inv
}

// CreateInvoice creates an invoice via the LND client and tracks it.
func (m *InvoiceManager) CreateInvoice(amountSats int64, memo string) (*TrackedInvoice, error) {
	if m.client == nil {
		return nil, fmt.Errorf("no LND client configured")
	}
	rHash, preq, err := m.client.CreateInvoice(amountSats, memo)
	if err != nil {
		return nil, err
	}
	inv := &TrackedInvoice{
		RHash:          rHash,
		PaymentRequest: preq,
		AmountSats:     amountSats,
		Memo:           memo,
		State:          InvoiceStateOpen,
		CreatedAt:      time.Now().Unix(),
	}
	m.mu.Lock()
	m.invoices[rHash] = inv
	m.mu.Unlock()
	return inv, nil
}

// Get returns the tracked invoice for rHash.
func (m *InvoiceManager) Get(rHash string) (*TrackedInvoice, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invoices[rHash]
	return inv, ok
}

// Settle marks an open invoice as settled.
func (m *InvoiceManager) Settle(rHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invoices[rHash]
	if !ok {
		return fmt.Errorf("invoice %s not found", rHash)
	}
	if inv.State != InvoiceStateOpen {
		return fmt.Errorf("invoice %s not open (state=%s)", rHash, inv.State)
	}
	inv.State = InvoiceStateSettled
	inv.SettledAt = time.Now().Unix()
	return nil
}

// Archive marks a settled invoice as archived (lifecycle complete).
func (m *InvoiceManager) Archive(rHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invoices[rHash]
	if !ok {
		return fmt.Errorf("invoice %s not found", rHash)
	}
	if inv.State != InvoiceStateSettled {
		return fmt.Errorf("invoice %s not settled (state=%s)", rHash, inv.State)
	}
	inv.State = InvoiceStateArchived
	inv.ArchivedAt = time.Now().Unix()
	return nil
}

// Expire marks an open invoice as expired.
func (m *InvoiceManager) Expire(rHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invoices[rHash]
	if !ok {
		return fmt.Errorf("invoice %s not found", rHash)
	}
	if inv.State != InvoiceStateOpen {
		return fmt.Errorf("invoice %s not open (state=%s)", rHash, inv.State)
	}
	inv.State = InvoiceStateExpired
	return nil
}

// Cancel marks an open invoice as canceled.
func (m *InvoiceManager) Cancel(rHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invoices[rHash]
	if !ok {
		return fmt.Errorf("invoice %s not found", rHash)
	}
	if inv.State != InvoiceStateOpen {
		return fmt.Errorf("invoice %s not open (state=%s)", rHash, inv.State)
	}
	inv.State = InvoiceStateCanceled
	return nil
}

// List returns all tracked invoices.
func (m *InvoiceManager) List() []*TrackedInvoice {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*TrackedInvoice, 0, len(m.invoices))
	for _, inv := range m.invoices {
		out = append(out, inv)
	}
	return out
}

// ListByState returns invoices in the given state.
func (m *InvoiceManager) ListByState(state InvoiceState) []*TrackedInvoice {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*TrackedInvoice, 0)
	for _, inv := range m.invoices {
		if inv.State == state {
			out = append(out, inv)
		}
	}
	return out
}

// InvoiceWebhookPayload is the body posted by an LND invoice-update webhook.
type InvoiceWebhookPayload struct {
	RHash   string `json:"r_hash"`
	Settled bool   `json:"settled"`
	State   string `json:"state,omitempty"`
}

// HandleInvoiceWebhook processes an invoice update, transitioning state and
// optionally running a paid callback (e.g. granting premium). It is idempotent:
// a settled invoice that is re-reported as settled is a no-op.
func (m *InvoiceManager) HandleInvoiceWebhook(payload InvoiceWebhookPayload, onPaid func(rHash string) error) (*TrackedInvoice, error) {
	if payload.Settled {
		if err := m.Settle(payload.RHash); err != nil {
			// Already settled is fine (idempotent webhook).
			inv, ok := m.Get(payload.RHash)
			if !ok || inv.State != InvoiceStateSettled {
				return nil, err
			}
		} else if onPaid != nil {
			if err := onPaid(payload.RHash); err != nil {
				return nil, err
			}
		}
	}
	inv, _ := m.Get(payload.RHash)
	return inv, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Payment history
// ──────────────────────────────────────────────────────────────────────────────

// PaymentHistory is a row in the payment_history table.
type PaymentHistory struct {
	ID         int64  `json:"id"`
	UserID     string `json:"user_id"`
	RHash      string `json:"r_hash"`
	AmountSats int64  `json:"amount_sats"`
	Type       string `json:"type"` // "subscription", "donation", "withdrawal", ...
	State      string `json:"state"`
	CreatedAt  int64  `json:"created_at"`
}

// Payment type constants.
const (
	PaymentTypeSubscription = "subscription"
	PaymentTypeDonation     = "donation"
	PaymentTypeWithdrawal   = "withdrawal"
)

// EnsurePaymentHistoryTable creates the payment_history table if it is missing.
func EnsurePaymentHistoryTable(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS payment_history (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     TEXT NOT NULL,
		r_hash      TEXT NOT NULL DEFAULT '',
		amount_sats INTEGER NOT NULL DEFAULT 0,
		type        TEXT NOT NULL,
		state       TEXT NOT NULL DEFAULT '',
		created_at  INTEGER NOT NULL
	)`)
	return err
}

// RecordPayment inserts a payment history row and fills in the assigned ID.
func RecordPayment(db *store.Store, ph *PaymentHistory) error {
	if err := EnsurePaymentHistoryTable(db); err != nil {
		return err
	}
	if ph.CreatedAt == 0 {
		ph.CreatedAt = time.Now().Unix()
	}
	res, err := db.DB().Exec(
		`INSERT INTO payment_history (user_id, r_hash, amount_sats, type, state, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		ph.UserID, ph.RHash, ph.AmountSats, ph.Type, ph.State, ph.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	ph.ID = id
	return nil
}

// GetPaymentHistory returns payment rows for a user, newest first.
func GetPaymentHistory(db *store.Store, userID string, limit int) ([]PaymentHistory, error) {
	if err := EnsurePaymentHistoryTable(db); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.DB().Query(
		`SELECT id, user_id, r_hash, amount_sats, type, state, created_at FROM payment_history WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PaymentHistory
	for rows.Next() {
		var ph PaymentHistory
		if err := rows.Scan(&ph.ID, &ph.UserID, &ph.RHash, &ph.AmountSats, &ph.Type, &ph.State, &ph.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ph)
	}
	return out, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Subscription payment (recurring)
// ──────────────────────────────────────────────────────────────────────────────

// renewalEntry is the metadata remembered for a pending subscription renewal.
type renewalEntry struct {
	UserID       string
	Tier         PremiumTier
	AmountSats   int64
	DurationDays int
	CreatedAt    int64
}

// SubscriptionPayment handles recurring subscription billing via Lightning.
type SubscriptionPayment struct {
	mu       sync.Mutex
	invoices *InvoiceManager
	pending  map[string]renewalEntry // rHash -> renewal metadata
}

// NewSubscriptionPayment builds a SubscriptionPayment on top of an InvoiceManager.
func NewSubscriptionPayment(im *InvoiceManager) *SubscriptionPayment {
	return &SubscriptionPayment{invoices: im, pending: make(map[string]renewalEntry)}
}

// CreateRenewal creates an invoice for renewing a user's subscription and
// remembers the renewal metadata so it can be applied on settlement.
func (sp *SubscriptionPayment) CreateRenewal(userID string, tier PremiumTier, amountSats int64, durationDays int) (*TrackedInvoice, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID required")
	}
	if amountSats <= 0 {
		return nil, fmt.Errorf("amountSats must be positive")
	}
	if durationDays <= 0 {
		return nil, fmt.Errorf("durationDays must be positive")
	}
	inv, err := sp.invoices.CreateInvoice(amountSats, "renewal:"+string(tier))
	if err != nil {
		return nil, err
	}
	sp.mu.Lock()
	sp.pending[inv.RHash] = renewalEntry{
		UserID:       userID,
		Tier:         tier,
		AmountSats:   amountSats,
		DurationDays: durationDays,
		CreatedAt:    time.Now().Unix(),
	}
	sp.mu.Unlock()
	return inv, nil
}

// PendingRenewals returns the pending renewal entries.
func (sp *SubscriptionPayment) PendingRenewals() []renewalEntry {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	out := make([]renewalEntry, 0, len(sp.pending))
	for _, e := range sp.pending {
		out = append(out, e)
	}
	return out
}

// SettleRenewal marks a renewal invoice settled and activates the premium
// subscription for the configured duration. If db is nil the premium activation
// is skipped (useful for tests that only check invoice state).
func (sp *SubscriptionPayment) SettleRenewal(rHash string, db *store.Store) error {
	sp.mu.Lock()
	entry, ok := sp.pending[rHash]
	sp.mu.Unlock()
	if !ok {
		return fmt.Errorf("no pending renewal for %s", rHash)
	}
	if err := sp.invoices.Settle(rHash); err != nil {
		return err
	}
	if db != nil {
		if err := ActivatePremium(db, entry.UserID, entry.Tier, entry.DurationDays); err != nil {
			return err
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Withdrawal requests
// ──────────────────────────────────────────────────────────────────────────────

// Withdrawal states.
const (
	WithdrawalStatePending    = "pending"
	WithdrawalStateProcessing = "processing"
	WithdrawalStateCompleted  = "completed"
	WithdrawalStateFailed     = "failed"
)

// WithdrawalRequest is a user's request to withdraw sats to an external destination.
type WithdrawalRequest struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	AmountSats int64  `json:"amount_sats"`
	Dest       string `json:"dest"` // Lightning payment request or on-chain address
	State      string `json:"state"`
	CreatedAt  int64  `json:"created_at"`
}

// WithdrawalManager tracks withdrawal requests in memory and (optionally) records
// them in the payment_history table.
type WithdrawalManager struct {
	mu       sync.Mutex
	requests map[string]*WithdrawalRequest
}

// NewWithdrawalManager creates an empty manager.
func NewWithdrawalManager() *WithdrawalManager {
	return &WithdrawalManager{requests: make(map[string]*WithdrawalRequest)}
}

// RequestWithdrawal creates a pending withdrawal. If db is non-nil it also
// records the request in payment_history.
func (wm *WithdrawalManager) RequestWithdrawal(db *store.Store, userID string, amountSats int64, dest string) (*WithdrawalRequest, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID required")
	}
	if amountSats <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if dest == "" {
		return nil, fmt.Errorf("destination required")
	}
	wr := &WithdrawalRequest{
		ID:         newRHash()[:16],
		UserID:     userID,
		AmountSats: amountSats,
		Dest:       dest,
		State:      WithdrawalStatePending,
		CreatedAt:  time.Now().Unix(),
	}
	wm.mu.Lock()
	wm.requests[wr.ID] = wr
	wm.mu.Unlock()

	if db != nil {
		_ = RecordPayment(db, &PaymentHistory{
			UserID:     userID,
			RHash:      wr.ID,
			AmountSats: amountSats,
			Type:       PaymentTypeWithdrawal,
			State:      WithdrawalStatePending,
		})
	}
	return wr, nil
}

// ProcessWithdrawal sends the withdrawal via the LND client and marks it
// completed (or failed on error). Without an LND client it is marked failed.
func (wm *WithdrawalManager) ProcessWithdrawal(client *LightningClient, id string) error {
	wm.mu.Lock()
	wr, ok := wm.requests[id]
	if !ok {
		wm.mu.Unlock()
		return fmt.Errorf("withdrawal %s not found", id)
	}
	if wr.State != WithdrawalStatePending {
		wm.mu.Unlock()
		return fmt.Errorf("withdrawal %s not pending (state=%s)", id, wr.State)
	}
	wr.State = WithdrawalStateProcessing
	wm.mu.Unlock()

	if client == nil {
		wm.setFailed(id)
		return fmt.Errorf("no LND client configured to process withdrawal")
	}
	if _, err := client.PayInvoice(wr.Dest); err != nil {
		wm.setFailed(id)
		return err
	}
	wm.mu.Lock()
	if wr.State == WithdrawalStateProcessing {
		wr.State = WithdrawalStateCompleted
	}
	wm.mu.Unlock()
	return nil
}

func (wm *WithdrawalManager) setFailed(id string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if wr, ok := wm.requests[id]; ok {
		wr.State = WithdrawalStateFailed
	}
}

// Get returns a withdrawal request by ID.
func (wm *WithdrawalManager) Get(id string) (*WithdrawalRequest, bool) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wr, ok := wm.requests[id]
	return wr, ok
}

// List returns all withdrawal requests.
func (wm *WithdrawalManager) List() []*WithdrawalRequest {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	out := make([]*WithdrawalRequest, 0, len(wm.requests))
	for _, wr := range wm.requests {
		out = append(out, wr)
	}
	return out
}
