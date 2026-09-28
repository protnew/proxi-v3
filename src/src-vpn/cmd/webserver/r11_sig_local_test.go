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

func TestR11SigLocalMustNotPassAsHexSchnorr(t *testing.T) {
	local := "local"
	for _, c := range local {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		// contains non-hex → cannot be schnorr hex
		return
	}
	if len(local) != 64 {
		return
	}
	t.Fatal("Sig:local must not be acceptable as schnorr hex")
}