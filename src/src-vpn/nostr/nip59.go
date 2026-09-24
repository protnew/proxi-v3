package nostr

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

const KindGiftWrap = 1059

// InvitePayload is the single R26b body for QR and rumor content.
type InvitePayload struct {
	Type       string `json:"type"`
	From       string `json:"from"`
	To         string `json:"to"`
	Onion      string `json:"onion,omitempty"`
	WtAddr     string `json:"wtAddr,omitempty"`
	WtCertHash string `json:"wtCertHash,omitempty"`
	Token      string `json:"token"`
	Exp        int64  `json:"exp"`
	Ts         int64  `json:"ts"`
	V          int    `json:"v"`
}

type nip59Seal struct {
	PubKey  string `json:"pubkey"`
	Content string `json:"content"`
}

// GiftWrapInvite seals the payload with NIP-44 (sender key) and wraps it
// again with an ephemeral key. The relay sees kind 1059 only.
func GiftWrapInvite(sender *btcec.PrivateKey, recipient *btcec.PublicKey, payload InvitePayload) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	if strings.Contains(string(raw), "wtAddr") && payload.WtAddr == "" {
		return Event{}, fmt.Errorf("empty wtAddr")
	}
	sealed, err := Encrypt44(sender, recipient, string(raw))
	if err != nil {
		return Event{}, err
	}
	sealRaw, err := json.Marshal(nip59Seal{
		PubKey:  hex.EncodeToString(sender.PubKey().SerializeCompressed()[1:]),
		Content: sealed,
	})
	if err != nil {
		return Event{}, err
	}
	eph, err := btcec.NewPrivateKey()
	if err != nil {
		return Event{}, err
	}
	wrapped, err := Encrypt44(eph, recipient, string(sealRaw))
	if err != nil {
		return Event{}, err
	}
	ev := Event{
		Kind:    KindGiftWrap,
		PubKey:  hex.EncodeToString(eph.PubKey().SerializeCompressed()[1:]),
		Content: wrapped,
		Tags:    [][]string{{"p", hex.EncodeToString(recipient.SerializeCompressed()[1:])}},
	}
	if payload.WtAddr != "" && strings.Contains(ev.Content, payload.WtAddr) {
		return Event{}, fmt.Errorf("plaintext wtAddr leaked into wrap")
	}
	return ev, nil
}

// OpenGiftWrap recovers the invite for the recipient.
func OpenGiftWrap(recipient *btcec.PrivateKey, ev Event) (InvitePayload, error) {
	if ev.Kind != KindGiftWrap {
		return InvitePayload{}, fmt.Errorf("kind %d is not gift-wrap", ev.Kind)
	}
	ephPub, err := xOnlyPub(ev.PubKey)
	if err != nil {
		return InvitePayload{}, err
	}
	sealJSON, err := Decrypt44(recipient, ephPub, ev.Content)
	if err != nil {
		return InvitePayload{}, err
	}
	var seal nip59Seal
	if err := json.Unmarshal([]byte(sealJSON), &seal); err != nil {
		return InvitePayload{}, err
	}
	senderPub, err := xOnlyPub(seal.PubKey)
	if err != nil {
		return InvitePayload{}, err
	}
	rumor, err := Decrypt44(recipient, senderPub, seal.Content)
	if err != nil {
		return InvitePayload{}, err
	}
	var payload InvitePayload
	if err := json.Unmarshal([]byte(rumor), &payload); err != nil {
		return InvitePayload{}, err
	}
	return payload, nil
}

func xOnlyPub(hexPub string) (*btcec.PublicKey, error) {
	b, err := hex.DecodeString(hexPub)
	if err != nil || len(b) != 32 {
		return nil, fmt.Errorf("bad x-only pubkey")
	}
	return schnorr.ParsePubKey(b)
}
