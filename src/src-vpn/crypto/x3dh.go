package crypto

// X3DH session bootstrap for Double Ratchet (E2EE-001).
// Separate from PreKeyBundle helpers in keyexchange.go (Ed25519-signed publish path).

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"

)

// X3DHKeyPair is an X25519 key pair.
type X3DHKeyPair struct {
	Priv [32]byte
	Pub  [32]byte
}

// GenerateX3DHKeyPair wraps GenerateDHKeyPair.
func GenerateX3DHKeyPair() (X3DHKeyPair, error) {
	priv, pub, err := GenerateDHKeyPair()
	return X3DHKeyPair{Priv: priv, Pub: pub}, err
}

// X3DHBobMaterial is Bob's private prekeys + public pubs.
type X3DHBobMaterial struct {
	IK  X3DHKeyPair
	SPK X3DHKeyPair
	OPK X3DHKeyPair
}

// NewX3DHBobMaterial creates Bob's X3DH keys.
func NewX3DHBobMaterial() (*X3DHBobMaterial, error) {
	ik, err := GenerateX3DHKeyPair()
	if err != nil {
		return nil, err
	}
	spk, err := GenerateX3DHKeyPair()
	if err != nil {
		return nil, err
	}
	opk, err := GenerateX3DHKeyPair()
	if err != nil {
		return nil, err
	}
	return &X3DHBobMaterial{IK: ik, SPK: spk, OPK: opk}, nil
}

// X3DHAliceOut is Alice initiate result.
type X3DHAliceOut struct {
	SharedSecret [32]byte
	EKPub        [32]byte
	IKPub        [32]byte
}

// X3DHInitiateAlice runs 4-DH X3DH (with OPK).
func X3DHInitiateAlice(aliceIK X3DHKeyPair, bob *X3DHBobMaterial) (X3DHAliceOut, error) {
	var out X3DHAliceOut
	out.IKPub = aliceIK.Pub
	ek, err := GenerateX3DHKeyPair()
	if err != nil {
		return out, err
	}
	out.EKPub = ek.Pub

	dh1, err := ComputeSharedSecret(aliceIK.Priv, bob.SPK.Pub)
	if err != nil {
		return out, err
	}
	dh2, err := ComputeSharedSecret(ek.Priv, bob.IK.Pub)
	if err != nil {
		return out, err
	}
	dh3, err := ComputeSharedSecret(ek.Priv, bob.SPK.Pub)
	if err != nil {
		return out, err
	}
	dh4, err := ComputeSharedSecret(ek.Priv, bob.OPK.Pub)
	if err != nil {
		return out, err
	}
	ikm := append(append(append(dh1[:], dh2[:]...), dh3[:]...), dh4[:]...)
	out.SharedSecret = kdfX3DH(ikm)
	return out, nil
}

// X3DHRespondBob derives the matching shared secret.
func X3DHRespondBob(bob *X3DHBobMaterial, aliceIKPub, aliceEKPub [32]byte) ([32]byte, error) {
	dh1, err := ComputeSharedSecret(bob.SPK.Priv, aliceIKPub)
	if err != nil {
		return [32]byte{}, err
	}
	dh2, err := ComputeSharedSecret(bob.IK.Priv, aliceEKPub)
	if err != nil {
		return [32]byte{}, err
	}
	dh3, err := ComputeSharedSecret(bob.SPK.Priv, aliceEKPub)
	if err != nil {
		return [32]byte{}, err
	}
	dh4, err := ComputeSharedSecret(bob.OPK.Priv, aliceEKPub)
	if err != nil {
		return [32]byte{}, err
	}
	ikm := append(append(append(dh1[:], dh2[:]...), dh3[:]...), dh4[:]...)
	return kdfX3DH(ikm), nil
}

// BootstrapDoubleRatchet runs X3DH then creates paired ratchet states.
func BootstrapDoubleRatchet(aliceIK X3DHKeyPair, bob *X3DHBobMaterial) (aliceRS, bobRS *RatchetState, err error) {
	aOut, err := X3DHInitiateAlice(aliceIK, bob)
	if err != nil {
		return nil, nil, err
	}
	skB, err := X3DHRespondBob(bob, aOut.IKPub, aOut.EKPub)
	if err != nil {
		return nil, nil, err
	}
	if aOut.SharedSecret != skB {
		return nil, nil, fmt.Errorf("x3dh shared secret mismatch")
	}
	aliceRS = NewRatchetState(aOut.SharedSecret[:], true)
	bobRS = NewRatchetState(skB[:], false)
	aliceRS.RemotePub = bobRS.DHPub
	bobRS.RemotePub = aliceRS.DHPub
	return aliceRS, bobRS, nil
}

func kdfX3DH(ikm []byte) [32]byte {
	mac := hmac.New(sha256.New, []byte("IndestructibleX3DH"))
	mac.Write(ikm)
	var out [32]byte
	copy(out[:], mac.Sum(nil))
	return out
}

