package store

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestMC1_RoundTripAndRawCipher(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	msg := Message{ID: "m1", From: "a", To: "b", Text: "service note", Encrypted: false, Timestamp: 1}
	if err := s.SaveMessage(msg); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := s.db.QueryRow(`SELECT text FROM messages WHERE id = ?`, "m1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, enc2Prefix) {
		t.Fatalf("raw=%q", raw)
	}
	if strings.Contains(raw, "service note") {
		t.Fatal("plaintext in column")
	}
	got, err := s.GetMessageByID("m1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "service note" {
		t.Fatalf("opened=%q", got.Text)
	}
}

func TestMC1_WrongKeyFailClosed(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SaveMessage(Message{ID: "m2", From: "a", To: "b", Text: "secret", Timestamp: 2}); err != nil {
		t.Fatal(err)
	}
	s.msgKey[0] ^= 0xff
	_, err = s.GetMessageByID("m2")
	if err == nil || !strings.Contains(err.Error(), "at-rest key mismatch") {
		t.Fatalf("err=%v", err)
	}
}

func TestMC1_LazyReseal(t *testing.T) {
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.Exec(`INSERT INTO messages (id, sender, recipient, text, encrypted, timestamp) VALUES ('old', 'a', 'b', 'legacy plain', 0, 3)`); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMessageByID("old")
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "legacy plain" {
		t.Fatal(got.Text)
	}
	var raw string
	if err := s.db.QueryRow(`SELECT text FROM messages WHERE id = 'old'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, enc2Prefix) {
		t.Fatalf("not resealed: %q", raw)
	}
}

func TestMC1_OSKeyNotRaw(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("dpapi")
	}
	raw := bytes.Repeat([]byte{9}, 32)
	blob, err := protectKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(blob, raw) || !bytes.HasPrefix(blob, []byte("DPAP")) {
		t.Fatal("key not dpapi-wrapped")
	}
	back, err := unprotectKey(blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, raw) {
		t.Fatal("roundtrip")
	}
}
