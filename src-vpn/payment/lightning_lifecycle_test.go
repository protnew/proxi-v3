package payment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Invoice lifecycle: create → settle → archive ───

func TestInvoiceLifecycle_CreateSettleArchive(t *testing.T) {
	// Mock LND for invoice creation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(InvoiceResponse{
			RHash:          "lifecyclehash123",
			PaymentRequest: "lnbc1000n1lifecycle",
			AddIndex:       "1",
		})
	}))
	defer srv.Close()

	client := NewLightningClient(srv.URL, "mac", "")
	mgr := NewInvoiceManager(client)

	// create
	inv, err := mgr.CreateInvoice(1000, "pro subscription")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if inv.State != InvoiceStateOpen {
		t.Errorf("after create state=%s, want open", inv.State)
	}
	if inv.RHash != "lifecyclehash123" {
		t.Errorf("rhash=%s", inv.RHash)
	}

	// settle
	if err := mgr.Settle(inv.RHash); err != nil {
		t.Fatalf("settle: %v", err)
	}
	got, _ := mgr.Get(inv.RHash)
	if got.State != InvoiceStateSettled {
		t.Errorf("after settle state=%s, want settled", got.State)
	}
	if got.SettledAt == 0 {
		t.Error("settled_at should be set")
	}

	// archive
	if err := mgr.Archive(inv.RHash); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got, _ = mgr.Get(inv.RHash)
	if got.State != InvoiceStateArchived {
		t.Errorf("after archive state=%s, want archived", got.State)
	}
	if got.ArchivedAt == 0 {
		t.Error("archived_at should be set")
	}
}

func TestInvoiceLifecycle_LocalOffline(t *testing.T) {
	mgr := NewInvoiceManager(nil) // offline
	inv := mgr.CreateLocalInvoice(500, "test")
	if inv.State != InvoiceStateOpen {
		t.Errorf("state=%s, want open", inv.State)
	}
	if inv.RHash == "" {
		t.Error("local invoice needs an rhash")
	}
	if err := mgr.Settle(inv.RHash); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if err := mgr.Archive(inv.RHash); err != nil {
		t.Fatalf("archive: %v", err)
	}
}

func TestInvoiceLifecycle_InvalidTransitions(t *testing.T) {
	mgr := NewInvoiceManager(nil)
	inv := mgr.CreateLocalInvoice(100, "x")

	// cannot archive an open invoice
	if err := mgr.Archive(inv.RHash); err == nil {
		t.Error("cannot archive an open invoice")
	}
	// cannot settle twice
	mgr.Settle(inv.RHash)
	if err := mgr.Settle(inv.RHash); err == nil {
		t.Error("cannot settle twice")
	}
	// unknown invoice
	if err := mgr.Settle("ghost"); err == nil {
		t.Error("expected error for unknown invoice")
	}
}

func TestInvoiceManager_CreateInvoice_NoClient(t *testing.T) {
	mgr := NewInvoiceManager(nil)
	if _, err := mgr.CreateInvoice(100, "x"); err == nil {
		t.Error("expected error without LND client")
	}
}

func TestInvoiceManager_ListAndFilter(t *testing.T) {
	mgr := NewInvoiceManager(nil)
	a := mgr.CreateLocalInvoice(10, "a")
	b := mgr.CreateLocalInvoice(20, "b")
	c := mgr.CreateLocalInvoice(30, "c")
	mgr.Settle(b.RHash)

	if len(mgr.List()) != 3 {
		t.Errorf("list len=%d, want 3", len(mgr.List()))
	}
	if len(mgr.ListByState(InvoiceStateOpen)) != 2 {
		t.Errorf("open count=%d, want 2", len(mgr.ListByState(InvoiceStateOpen)))
	}
	if len(mgr.ListByState(InvoiceStateSettled)) != 1 {
		t.Errorf("settled count=%d, want 1", len(mgr.ListByState(InvoiceStateSettled)))
	}
	_ = a
	_ = c
}

func TestInvoice_ExpireCancel(t *testing.T) {
	mgr := NewInvoiceManager(nil)
	open := mgr.CreateLocalInvoice(10, "o")
	if err := mgr.Expire(open.RHash); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if got, _ := mgr.Get(open.RHash); got.State != InvoiceStateExpired {
		t.Errorf("state=%s, want expired", got.State)
	}
	open2 := mgr.CreateLocalInvoice(10, "o2")
	if err := mgr.Cancel(open2.RHash); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got, _ := mgr.Get(open2.RHash); got.State != InvoiceStateCanceled {
		t.Errorf("state=%s, want canceled", got.State)
	}
}

// ─── Invoice webhook ───

func TestHandleInvoiceWebhook_Settle(t *testing.T) {
	mgr := NewInvoiceManager(nil)
	inv := mgr.CreateLocalInvoice(1000, "wh")
	paid := false
	got, err := mgr.HandleInvoiceWebhook(InvoiceWebhookPayload{
		RHash: inv.RHash, Settled: true,
	}, func(rHash string) error {
		paid = true
		return nil
	})
	if err != nil {
		t.Fatalf("webhook: %v", err)
	}
	if got.State != InvoiceStateSettled {
		t.Errorf("state=%s, want settled", got.State)
	}
	if !paid {
		t.Error("onPaid callback should have run")
	}
}

func TestHandleInvoiceWebhook_Idempotent(t *testing.T) {
	mgr := NewInvoiceManager(nil)
	inv := mgr.CreateLocalInvoice(1000, "wh")
	calls := 0
	cb := func(rHash string) error { calls++; return nil }
	mgr.HandleInvoiceWebhook(InvoiceWebhookPayload{RHash: inv.RHash, Settled: true}, cb)
	mgr.HandleInvoiceWebhook(InvoiceWebhookPayload{RHash: inv.RHash, Settled: true}, cb)
	if calls != 1 {
		t.Errorf("onPaid should fire once (idempotent), got %d", calls)
	}
}

// ─── Subscription renewal ───

func TestSubscriptionRenewal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(InvoiceResponse{
			RHash:          "renewalhash",
			PaymentRequest: "lnbc5000n1renewal",
			AddIndex:       "9",
		})
	}))
	defer srv.Close()

	db := newTestDB(t)
	client := NewLightningClient(srv.URL, "mac", "")
	mgr := NewInvoiceManager(client)
	sp := NewSubscriptionPayment(mgr)

	// Create a renewal invoice.
	inv, err := sp.CreateRenewal("alice", TierPro, 5000, 30)
	if err != nil {
		t.Fatalf("create renewal: %v", err)
	}
	if inv.State != InvoiceStateOpen {
		t.Errorf("renewal state=%s, want open", inv.State)
	}
	if len(sp.PendingRenewals()) != 1 {
		t.Errorf("expected 1 pending renewal, got %d", len(sp.PendingRenewals()))
	}

	// Before settlement the user is on free tier.
	sub, _ := GetSubscription(db, "alice")
	if sub.Tier != TierFree {
		t.Errorf("before settlement tier=%s, want free", sub.Tier)
	}

	// Settle → activates premium.
	if err := sp.SettleRenewal(inv.RHash, db); err != nil {
		t.Fatalf("settle renewal: %v", err)
	}
	got, _ := mgr.Get(inv.RHash)
	if got.State != InvoiceStateSettled {
		t.Errorf("after settle state=%s, want settled", got.State)
	}
	sub, _ = GetSubscription(db, "alice")
	if sub.Tier != TierPro {
		t.Errorf("after settlement tier=%s, want pro", sub.Tier)
	}
	if sub.ExpiresAt == 0 {
		t.Error("expiry should be set")
	}
}

func TestSubscriptionRenewal_UnknownInvoice(t *testing.T) {
	mgr := NewInvoiceManager(nil)
	sp := NewSubscriptionPayment(mgr)
	if err := sp.SettleRenewal("nope", nil); err == nil {
		t.Error("expected error for unknown renewal")
	}
}

func TestSubscriptionRenewal_Validation(t *testing.T) {
	mgr := NewInvoiceManager(nil)
	sp := NewSubscriptionPayment(mgr)
	if _, err := sp.CreateRenewal("", TierPro, 100, 30); err == nil {
		t.Error("empty userID should error")
	}
	if _, err := sp.CreateRenewal("u", TierPro, 0, 30); err == nil {
		t.Error("zero amount should error")
	}
	if _, err := sp.CreateRenewal("u", TierPro, 100, 0); err == nil {
		t.Error("zero duration should error")
	}
}

// ─── Payment history ───

func TestPaymentHistory(t *testing.T) {
	db := newTestDB(t)
	if err := RecordPayment(db, &PaymentHistory{
		UserID: "alice", RHash: "h1", AmountSats: 1000,
		Type: PaymentTypeSubscription, State: "settled",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := RecordPayment(db, &PaymentHistory{
		UserID: "alice", RHash: "h2", AmountSats: 250,
		Type: PaymentTypeDonation, State: "settled",
	}); err != nil {
		t.Fatalf("record2: %v", err)
	}
	if err := RecordPayment(db, &PaymentHistory{
		UserID: "bob", RHash: "h3", AmountSats: 500,
		Type: PaymentTypeSubscription, State: "settled",
	}); err != nil {
		t.Fatalf("record3: %v", err)
	}

	hist, err := GetPaymentHistory(db, "alice", 10)
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("alice history len=%d, want 2", len(hist))
	}
	for _, h := range hist {
		if h.UserID != "alice" {
			t.Errorf("user mismatch: %s", h.UserID)
		}
	}
}

func TestPaymentHistory_TableIdempotent(t *testing.T) {
	db := newTestDB(t)
	EnsurePaymentHistoryTable(db)
	EnsurePaymentHistoryTable(db) // twice must not error
	RecordPayment(db, &PaymentHistory{UserID: "u", Type: PaymentTypeDonation, AmountSats: 1})
	hist, _ := GetPaymentHistory(db, "u", 1)
	if len(hist) != 1 {
		t.Errorf("len=%d, want 1", len(hist))
	}
}

// ─── Withdrawal ───

func TestWithdrawal_NoClient_Fails(t *testing.T) {
	db := newTestDB(t)
	wm := NewWithdrawalManager()
	wr, err := wm.RequestWithdrawal(db, "alice", 1000, "lnbc1000withdrawaldestination!!")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if wr.State != WithdrawalStatePending {
		t.Errorf("state=%s, want pending", wr.State)
	}
	// No LND client → fails.
	if err := wm.ProcessWithdrawal(nil, wr.ID); err == nil {
		t.Error("expected failure without LND client")
	}
	got, _ := wm.Get(wr.ID)
	if got.State != WithdrawalStateFailed {
		t.Errorf("state=%s, want failed", got.State)
	}
	// Recorded in payment history.
	hist, _ := GetPaymentHistory(db, "alice", 10)
	if len(hist) != 1 {
		t.Errorf("expected 1 payment history row, got %d", len(hist))
	}
}

func TestWithdrawal_Processed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := newTestDB(t)
	client := NewLightningClient(srv.URL, "mac", "")
	wm := NewWithdrawalManager()
	wr, err := wm.RequestWithdrawal(db, "alice", 500, "0123456789abcdef0123456789abcdef_dest")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := wm.ProcessWithdrawal(client, wr.ID); err != nil {
		t.Fatalf("process: %v", err)
	}
	got, _ := wm.Get(wr.ID)
	if got.State != WithdrawalStateCompleted {
		t.Errorf("state=%s, want completed", got.State)
	}
}

func TestWithdrawal_Validation(t *testing.T) {
	wm := NewWithdrawalManager()
	if _, err := wm.RequestWithdrawal(nil, "", 100, "dest"); err == nil {
		t.Error("empty userID should error")
	}
	if _, err := wm.RequestWithdrawal(nil, "u", 0, "dest"); err == nil {
		t.Error("zero amount should error")
	}
	if _, err := wm.RequestWithdrawal(nil, "u", 100, ""); err == nil {
		t.Error("empty dest should error")
	}
}

func TestWithdrawal_NotFound(t *testing.T) {
	wm := NewWithdrawalManager()
	if err := wm.ProcessWithdrawal(nil, "ghost"); err == nil {
		t.Error("expected error for unknown withdrawal")
	}
}

// ─── Health check ───

func TestHealthCheck_Online(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(GetInfoResponse{Version: "0.18.0", Alias: "test-node", Synced: true})
	}))
	defer srv.Close()

	lc := NewLightningClient(srv.URL, "mac", "")
	h := lc.HealthCheck()
	if !h.Online {
		t.Errorf("expected online, error=%s", h.Error)
	}
	if h.Version != "0.18.0" {
		t.Errorf("version=%s", h.Version)
	}
	if h.Alias != "test-node" {
		t.Errorf("alias=%s", h.Alias)
	}
	if h.Latency <= 0 {
		t.Error("latency should be positive")
	}
}

func TestHealthCheck_Offline(t *testing.T) {
	lc := NewLightningClient("http://127.0.0.1:1", "mac", "")
	lc.client.Timeout = 1
	h := lc.HealthCheck()
	if h.Online {
		t.Error("expected offline")
	}
	if h.Error == "" {
		t.Error("expected an error message")
	}
}

func TestHealthCheck_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	lc := NewLightningClient(srv.URL, "mac", "")
	h := lc.HealthCheck()
	if h.Online {
		t.Error("expected offline for 500")
	}
}
