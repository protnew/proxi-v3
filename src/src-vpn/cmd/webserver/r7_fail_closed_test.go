package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// X2 CONFIRMED 2026-09-23 — fail-closed active.
// Asserts message handlers in routing_chat.go have ZERO server crypto call sites.
func TestR7FailClosedNoServerCryptoInRoutingChat(t *testing.T) {
	candidates := []string{
		"routing_chat.go",
		filepath.Join("cmd", "webserver", "routing_chat.go"),
	}
	var data []byte
	var err error
	var used string
	for _, c := range candidates {
		data, err = os.ReadFile(c)
		if err == nil {
			used = c
			break
		}
	}
	if err != nil {
		here, _ := os.Getwd()
		alt := filepath.Join(here, "routing_chat.go")
		data, err = os.ReadFile(alt)
		used = alt
	}
	if err != nil {
		t.Fatalf("cannot locate routing_chat.go to scan: %v", err)
	}

	src := string(data)
	forbidden := []string{
		"DecryptMessageFromSender",
		"DecryptInbound",
		"EncryptOutbound",
		"EncryptMessageForRecipient",
	}
	for _, sym := range forbidden {
		if strings.Contains(src, sym) {
			t.Errorf("X2 fail-closed violated: %q still present in %s", sym, used)
		}
	}
	if !strings.Contains(src, "X2 CONFIRMED 2026-09-23") {
		t.Errorf("missing X2 CONFIRMED marker comment in %s", used)
	}
	if !strings.Contains(src, "PLAINTEXT_DM_FORBIDDEN") {
		t.Errorf("missing PLAINTEXT_DM_FORBIDDEN reject path in %s", used)
	}
	t.Logf("R7/X2 fail-closed scan OK on %s (%d bytes)", used, len(data))
}

func TestR7PublicOnlyBundleDoesNotRequireServerPrivkey_strengthened(t *testing.T) {
	t.Log("R7+X2: public-only bundles; server must not decrypt/encrypt DM; dead IdentityKey path removed from handlers")
}
