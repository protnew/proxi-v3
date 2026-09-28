package economy

import (
	"encoding/json"
	"fmt"
)

// Table #60 winner: ProxiBit — internal unit, not Bitcoin, not a listed token.
// 1 ProxiBit = 1_000_000 bits transferred or stored. Ledger = Nostr kind 30091.

const BitsPerProxiBit int64 = 1_000_000
const ProxiBitKind = 30091

type ProxiBit int64

type BitRecord struct {
	PeerID     string
	BitsTx     int64
	BitsStored int64
	Unit       ProxiBit
	Kind       int
}

func BitsToProxiBit(bits int64) ProxiBit {
	if bits < 0 {
		return 0
	}
	return ProxiBit(bits / BitsPerProxiBit)
}

func RecordFromBytes(peer string, bytesTx, bytesStored int64) BitRecord {
	bitsTx := bytesTx * 8
	bitsSt := bytesStored * 8
	return BitRecord{
		PeerID:     peer,
		BitsTx:     bitsTx,
		BitsStored: bitsSt,
		Unit:       BitsToProxiBit(bitsTx + bitsSt),
		Kind:       ProxiBitKind,
	}
}

func (r BitRecord) CanMint() bool {
	measured := r.BitsTx + r.BitsStored
	if measured <= 0 {
		return false
	}
	return r.Unit == BitsToProxiBit(measured) && r.Unit > 0
}

func (r BitRecord) NostrEvent(payer string) map[string]any {
	payload := map[string]any{
		"unit":        "proxibit",
		"bits_tx":     r.BitsTx,
		"bits_stored": r.BitsStored,
		"amount":      int64(r.Unit),
		"peer":        r.PeerID,
	}
	raw, _ := json.Marshal(payload)
	return map[string]any{
		"kind":    ProxiBitKind,
		"pubkey":  payer,
		"content": string(raw),
		"tags":    [][]string{{"d", fmt.Sprintf("proxibit:%s", r.PeerID)}},
	}
}

// ProtocolLocks — table #60 legal=7 only if these stay on.
type Locks struct {
	NoP2P           bool
	NoRedeem        bool
	NoSale          bool
	MintOnlyMeasured bool
	BurnOnConsume   bool
}

func ProtocolLocks() Locks {
	return Locks{NoP2P: true, NoRedeem: true, NoSale: true, MintOnlyMeasured: true, BurnOnConsume: true}
}

var (
	ErrP2PForbidden    = fmt.Errorf("proxibit: P2P transfer forbidden")
	ErrRedeemForbidden = fmt.Errorf("proxibit: redeem to money forbidden")
	ErrSaleForbidden   = fmt.Errorf("proxibit: sale of unit forbidden")
	ErrQuota           = fmt.Errorf("proxibit: not enough quota")
)

func Transfer(_ BitRecord, _ string, _ ProxiBit) error { return ErrP2PForbidden }
func Redeem(_ BitRecord, _ string) error               { return ErrRedeemForbidden }
func Sell(_ BitRecord, _ ProxiBit, _ string) error     { return ErrSaleForbidden }

func Burn(r BitRecord, n ProxiBit) (BitRecord, error) {
	if n <= 0 || r.Unit < n {
		return r, ErrQuota
	}
	r.Unit -= n
	return r, nil
}

// SettleNode burns user quota for a node service. Not a P2P transfer of the unit.
func SettleNode(r BitRecord, _ string, n ProxiBit) (BitRecord, error) {
	return Burn(r, n)
}
