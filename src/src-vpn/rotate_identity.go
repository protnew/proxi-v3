//go:build ignore

package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/unkillable-messenger/vpn/identity"
	"github.com/unkillable-messenger/vpn/store"
)

func fp(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum[:8])
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run rotate_identity.go <messenger.db>")
		os.Exit(1)
	}
	dbPath := os.Args[1]
	s, err := store.NewStore(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer s.Close()
	oldNpub, oldNsec, _, err := s.LoadIdentity()
	if err == nil {
		fmt.Printf("OLD_NSEC_LEN=%d OLD_NSEC_SHA256_16=%s OLD_NPUB_LEN=%d\n", len(oldNsec), fp(oldNsec), len(oldNpub))
	} else {
		fmt.Println("OLD_IDENTITY=absent")
	}
	priv, _, err := identity.GenerateKeyPair()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	nsec := identity.PrivKeyToNsec(priv)
	npub := identity.PubKeyToNpub(priv.PubKey())
	mnemonic, _ := identity.GenerateMnemonic()
	if err := s.SaveIdentity(npub, nsec, mnemonic); err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}
	fmt.Printf("NEW_NSEC_LEN=%d NEW_NSEC_SHA256_16=%s NEW_NPUB_LEN=%d MATCH_OLD=%v\n", len(nsec), fp(nsec), len(npub), nsec == oldNsec)
}
