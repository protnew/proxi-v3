// P1 (2026-09-19): challenge-response auth — JWT выдаётся только после свежей
// schnorr-подписи server-нонса. Былая схема «голый npub → токен» = захват аккаунта.
// Протокол: POST /api/auth/challenge {npub} → {challenge}
//           POST /api/auth/signup|login {npub, event} — event = NIP-42-style
//           kind:22242, tag ["challenge", <c>], подписан ключом npub.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil/bech32"
)

const (
	authChallengeTTL = 5 * time.Minute
	authEventKind    = 22242
	authEventMaxSkew = 10 * time.Minute
)

type authChallenge struct {
	value   string
	expires time.Time
}

var authChallenges = struct {
	sync.Mutex
	m map[string]authChallenge
}{m: map[string]authChallenge{}}

func issueAuthChallenge(npub string) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	c := authChallenge{value: hex.EncodeToString(b), expires: time.Now().Add(authChallengeTTL)}
	authChallenges.Lock()
	now := time.Now()
	for k, v := range authChallenges.m {
		if now.After(v.expires) {
			delete(authChallenges.m, k)
		}
	}
	authChallenges.m[npub] = c
	authChallenges.Unlock()
	return c.value, c.expires, nil
}

// consumeAuthChallenge — single-use: валидный нонс удаляется.
func consumeAuthChallenge(npub, challenge string) bool {
	authChallenges.Lock()
	defer authChallenges.Unlock()
	c, ok := authChallenges.m[npub]
	if !ok || time.Now().After(c.expires) || c.value != challenge {
		return false
	}
	delete(authChallenges.m, npub)
	return true
}

// nostrAuthEvent — подмножество NIP-01 события для проверки подписи.
type nostrAuthEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

// jsString сериализует строку как JSON.stringify (UTF-8 без \u-эскейпа не-ASCII).
func jsString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// nostrEventID = sha256(NIP-01 serialization [0,pubkey,created_at,kind,tags,content]).
func nostrEventID(ev *nostrAuthEvent) ([32]byte, error) {
	var sb strings.Builder
	sb.WriteString("[0,")
	sb.WriteString(jsString(ev.PubKey))
	sb.WriteByte(',')
	sb.WriteString(strconv.FormatInt(ev.CreatedAt, 10))
	sb.WriteByte(',')
	sb.WriteString(strconv.Itoa(ev.Kind))
	sb.WriteByte(',')
	tagsJSON, err := json.Marshal(ev.Tags)
	if err != nil {
		return [32]byte{}, err
	}
	// Go Marshal эскейпит <>& как <> — JS JSON.stringify нет
	tagStr := strings.ReplaceAll(string(tagsJSON), "\\u003c", "<")
	tagStr = strings.ReplaceAll(tagStr, "\\u003e", ">")
	tagStr = strings.ReplaceAll(tagStr, "\\u0026", "&")
	sb.WriteString(tagStr)
	sb.WriteByte(',')
	sb.WriteString(jsString(ev.Content))
	sb.WriteByte(']')
	return sha256.Sum256([]byte(sb.String())), nil
}

// npubToXOnlyHex: npub в проекте бывает 33B-compressed (legacy Go) или
// 32B x-only (стандарт Nostr, PWA). Также принимаем сырой 64-hex pubkey.
func npubToXOnlyHex(npub string) (string, error) {
	if len(npub) == 64 {
		if b, err := hex.DecodeString(npub); err == nil && len(b) == 32 {
			return npub, nil
		}
	}
	hrp, data, err := bech32.DecodeToBase256(npub)
	if err != nil {
		return "", fmt.Errorf("decode npub: %w", err)
	}
	if hrp != "npub" {
		return "", errors.New("invalid hrp: expected npub")
	}
	switch len(data) {
	case 32:
		return hex.EncodeToString(data), nil
	case 33:
		pk, err := btcec.ParsePubKey(data)
		if err != nil {
			return "", fmt.Errorf("parse compressed pubkey: %w", err)
		}
		return hex.EncodeToString(pk.SerializeCompressed()[1:]), nil
	}
	return "", fmt.Errorf("unexpected npub payload: %d bytes", len(data))
}

// verifyAuthEvent проверяет подписанное событие и поглощает challenge.
func verifyAuthEvent(npub, challenge string, ev *nostrAuthEvent) error {
	if ev == nil {
		return errors.New("signed auth event required")
	}
	if ev.Kind != authEventKind {
		return fmt.Errorf("kind must be %d", authEventKind)
	}
	now := time.Now().Unix()
	if ev.CreatedAt < now-int64(authEventMaxSkew.Seconds()) || ev.CreatedAt > now+int64(authEventMaxSkew.Seconds()) {
		return errors.New("created_at out of allowed skew")
	}
	found := ev.Content == challenge
	for _, t := range ev.Tags {
		if len(t) >= 2 && t[0] == "challenge" && t[1] == challenge {
			found = true
		}
	}
	if !found {
		return errors.New("challenge mismatch")
	}
	pkHex, err := npubToXOnlyHex(npub)
	if err != nil {
		return err
	}
	if !strings.EqualFold(ev.PubKey, pkHex) {
		return errors.New("event pubkey does not match npub")
	}
	id, err := nostrEventID(ev)
	if err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(id[:]), ev.ID) {
		return errors.New("event id mismatch")
	}
	sigBytes, err := hex.DecodeString(ev.Sig)
	if err != nil {
		return errors.New("bad sig hex")
	}
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return errors.New("bad schnorr sig")
	}
	pkBytes, err := hex.DecodeString(ev.PubKey)
	if err != nil {
		return errors.New("bad pubkey hex")
	}
	pk, err := schnorr.ParsePubKey(pkBytes)
	if err != nil {
		return errors.New("bad x-only pubkey")
	}
	if !sig.Verify(id[:], pk) {
		return errors.New("signature verification failed")
	}
	if !consumeAuthChallenge(npub, challenge) {
		return errors.New("challenge unknown, expired or already used")
	}
	return nil
}

// handleAuthChallenge — POST /api/auth/challenge {npub} → {challenge, expires_at}
func (s *Server) handleAuthChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}
	var body struct {
		Npub string `json:"npub"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if _, err := npubToXOnlyHex(body.Npub); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_NPUB", err.Error())
		return
	}
	c, exp, err := issueAuthChallenge(body.Npub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"challenge":  c,
		"expires_at": exp.Unix(),
	})
}
