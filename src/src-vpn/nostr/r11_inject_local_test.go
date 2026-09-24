package nostr

import (
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

type r11MockDB struct {
	saves int
}

func (m *r11MockDB) SaveNostrEvent(evt store.NostrEvent) error {
	m.saves++
	return nil
}

func (m *r11MockDB) GetNostrEvents(filter store.NostrEventFilter) ([]store.NostrEvent, error) {
	return nil, nil
}

func TestVerifyEventSigRejectsLocalMarker(t *testing.T) {
	err := verifyEventSig(&Event{
		ID:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PubKey: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Sig:    "local",
	})
	if err == nil {
		t.Fatal("verifyEventSig must reject Sig=local")
	}
}

func TestInjectLocalEventSigLocalDoesNotWriteDB(t *testing.T) {
	db := &r11MockDB{}
	r := NewRelay(10, db)
	r.InjectLocalEvent(Event{PubKey: "ab", Kind: 1, Content: "x", Sig: "local"})
	if db.saves != 0 {
		t.Fatalf("SaveNostrEvent called %d times, want 0 for Sig=local", db.saves)
	}
}
