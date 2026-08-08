package crypto

import "testing"

func TestX3DHSharedSecretMatch(t *testing.T) {
	t.Parallel()
	bob, err := NewX3DHBobMaterial()
	if err != nil {
		t.Fatal(err)
	}
	aliceIK, err := GenerateX3DHKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	aOut, err := X3DHInitiateAlice(aliceIK, bob)
	if err != nil {
		t.Fatal(err)
	}
	skB, err := X3DHRespondBob(bob, aOut.IKPub, aOut.EKPub)
	if err != nil {
		t.Fatal(err)
	}
	if aOut.SharedSecret != skB {
		t.Fatalf("mismatch\na=%x\nb=%x", aOut.SharedSecret, skB)
	}
}

func TestX3DHThenDoubleRatchet(t *testing.T) {
	t.Parallel()
	bob, err := NewX3DHBobMaterial()
	if err != nil {
		t.Fatal(err)
	}
	aliceIK, err := GenerateX3DHKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	aliceRS, bobRS, err := BootstrapDoubleRatchet(aliceIK, bob)
	if err != nil {
		t.Fatal(err)
	}
	ct, err := aliceRS.RatchetEncrypt([]byte("X3DH+DR hello"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := bobRS.RatchetDecrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != "X3DH+DR hello" {
		t.Fatalf("got %q", pt)
	}
	// reverse
	ct2, err := bobRS.RatchetEncrypt([]byte("ack"))
	if err != nil {
		t.Fatal(err)
	}
	pt2, err := aliceRS.RatchetDecrypt(ct2)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt2) != "ack" {
		t.Fatalf("got %q", pt2)
	}
}
