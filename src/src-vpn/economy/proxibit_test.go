package economy

import "testing"

func TestBitsToProxiBit_OneMegabit(t *testing.T) {
	if BitsToProxiBit(1_000_000) != 1 {
		t.Fatalf("1 Mbit must be 1 ProxiBit, got %d", BitsToProxiBit(1_000_000))
	}
	if BitsToProxiBit(999_999) != 0 {
		t.Fatal("less than 1 Mbit is 0 units (no rounding up)")
	}
}

func TestRecordFromBytes_CountsBitsNotBytes(t *testing.T) {
	r := RecordFromBytes("npub1peer", 125_000, 0) // 125_000 bytes = 1_000_000 bits
	if r.BitsTx != 1_000_000 {
		t.Fatalf("bits tx %d", r.BitsTx)
	}
	if r.Unit != 1 {
		t.Fatalf("unit %d want 1", r.Unit)
	}
	if r.Kind != 30091 {
		t.Fatalf("kind %d want 30091 (Nostr state, table #39)", r.Kind)
	}
}

func TestCannotMintFromAir(t *testing.T) {
	empty := BitRecord{PeerID: "x", Unit: 100}
	if empty.CanMint() {
		t.Fatal("cannot mint ProxiBit without measured bits")
	}
	ok := RecordFromBytes("x", 250_000, 0) // 2 Mbit
	if !ok.CanMint() || ok.Unit != 2 {
		t.Fatalf("measured traffic must mint, got can=%v unit=%d", ok.CanMint(), ok.Unit)
	}
}

func TestNostrEventShape(t *testing.T) {
	r := RecordFromBytes("npub1abc", 125_000, 125_000)
	ev := r.NostrEvent("npub1payer")
	if ev["kind"] != 30091 {
		t.Fatalf("kind %v", ev["kind"])
	}
	if ev["pubkey"] != "npub1payer" {
		t.Fatal("payer pubkey missing")
	}
	content, _ := ev["content"].(string)
	if content == "" || !containsAll(content, "proxibit", "bits_tx", "bits_stored") {
		t.Fatalf("content %q", content)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestTransferP2PForbidden(t *testing.T) {
	a := RecordFromBytes("npub1a", 250_000, 0) // 2 units
	if !a.CanMint() {
		t.Fatal("setup")
	}
	err := Transfer(a, "npub1b", 1)
	if err == nil {
		t.Fatal("P2P transfer of ProxiBit must be forbidden")
	}
}

func TestRedeemToMoneyForbidden(t *testing.T) {
	a := RecordFromBytes("npub1a", 250_000, 0)
	if Redeem(a, "USD") == nil {
		t.Fatal("cannot redeem ProxiBit to fiat/BTC")
	}
	if Redeem(a, "BTC") == nil {
		t.Fatal("cannot redeem ProxiBit to BTC")
	}
}

func TestSellUnitForbidden(t *testing.T) {
	a := RecordFromBytes("npub1a", 250_000, 0)
	if Sell(a, 1, "sat") == nil {
		t.Fatal("cannot sell ProxiBit as a coin")
	}
}

func TestBurnOnConsumeReducesQuota(t *testing.T) {
	a := RecordFromBytes("npub1a", 375_000, 0) // 3 Mbit = 3 units
	left, err := Burn(a, 1)
	if err != nil {
		t.Fatalf("burn service quota: %v", err)
	}
	if left.Unit != 2 {
		t.Fatalf("quota after burn want 2 got %d", left.Unit)
	}
	if _, err := Burn(left, 5); err == nil {
		t.Fatal("cannot burn more than remaining quota")
	}
}

func TestNodeSettlementIsBurnNotTransfer(t *testing.T) {
	quota := RecordFromBytes("npub1user", 250_000, 0)
	left, err := SettleNode(quota, "npub1node", 1)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if left.Unit != 1 {
		t.Fatalf("user quota after node settle want 1 got %d", left.Unit)
	}
	if Transfer(quota, "npub1node", 1) == nil {
		t.Fatal("settlement must not be a P2P transfer")
	}
}

func TestProtocolLocksDeclared(t *testing.T) {
	l := ProtocolLocks()
	if !l.NoP2P || !l.NoRedeem || !l.NoSale || !l.MintOnlyMeasured || !l.BurnOnConsume {
		t.Fatalf("locks incomplete: %+v", l)
	}
}
