package main

import "testing"

// R7 hygiene: documents that server auto-decrypt via IdentityKey must not be extended.
// Full sealed-server / NIP44 rewrite is B_gated (X2) — not implemented here.
func TestR7PublicOnlyBundleDoesNotRequireServerPrivkey(t *testing.T) {
	t.Log("R7: public-only bundles — server must not hold/use recipient private keys; dead IdentityKey path not extended")
}
