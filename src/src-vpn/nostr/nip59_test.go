package nostr

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
)

func TestR26b_GiftWrapHidesWtAddr(t *testing.T) {
	sender, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	from := hex.EncodeToString(sender.PubKey().SerializeCompressed()[1:])
	to := hex.EncodeToString(recipient.PubKey().SerializeCompressed()[1:])
	now := time.Now().Unix()
	payload := InvitePayload{
		Type: "vpn_invite", From: from, To: to,
		WtAddr: "203.0.113.9:4433", Token: "tok", Exp: now + 900, Ts: now, V: 1,
	}
	ev, err := GiftWrapInvite(sender, recipient.PubKey(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Kind != 1059 {
		t.Fatalf("kind %d", ev.Kind)
	}
	if strings.Contains(ev.Content, "203.0.113.9") {
		t.Fatal("wtAddr readable by relay")
	}
	got, err := OpenGiftWrap(recipient, ev)
	if err != nil {
		t.Fatal(err)
	}
	if got.WtAddr != payload.WtAddr || got.Token != "tok" {
		t.Fatalf("roundtrip %#v", got)
	}
	_, err = OpenGiftWrap(sender, ev)
	if err == nil {
		t.Fatal("third party must not open the wrap")
	}
}

func TestOpenGiftWrap_RejectsForgedFrom(t *testing.T) {
	sender, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	payload := InvitePayload{
		Type: "vpn_invite", From: "aa", To: "bb",
		WtAddr: "203.0.113.9:4433", Token: "tok", Exp: 99, Ts: 1, V: 1,
	}
	ev, err := GiftWrapInvite(sender, recipient.PubKey(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenGiftWrap(recipient, ev); err == nil {
		t.Fatal("forged from accepted")
	}
}
