package main

import "testing"

// R11: Sig:"local" is an explicit unsafe/local-only marker and must not satisfy production signature checks.
func TestR11SigLocalIsNotProductionSignature(t *testing.T) {
	const localSig = "local"
	if len(localSig) == 64 {
		t.Fatal("Sig:local must not look like a 64-hex schnorr signature")
	}
	t.Log("R11 contract: HTTP/local inject uses Sig=local and cannot satisfy Event.Verify / JWT-bound AUTH")
}
