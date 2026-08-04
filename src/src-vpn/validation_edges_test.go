package vpn

import (
	"strings"
	"testing"
)

func TestValidateSignupInput_Table(t *testing.T) {
	cases := []struct {
		name, npub string
		wantErr    bool
	}{
		{"ok", "npub1abc", false},
		{"", "npub1abc", false},
		{strings.Repeat("n", MaxNameLen), "npub1", false},
		{strings.Repeat("n", MaxNameLen+1), "npub1", true},
		{"ok", strings.Repeat("p", MaxPubKeyLen), false},
		{"ok", strings.Repeat("p", MaxPubKeyLen+1), true},
	}
	for i, tc := range cases {
		err := ValidateSignupInput(tc.name, tc.npub)
		if tc.wantErr && err == nil {
			t.Fatalf("case %d: want err", i)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("case %d: unexpected %v", i, err)
		}
	}
}

func TestValidateMessage_Table(t *testing.T) {
	if err := ValidateMessage(""); err != nil {
		t.Fatalf("empty ok: %v", err)
	}
	if err := ValidateMessage(strings.Repeat("z", MaxMessageLen)); err != nil {
		t.Fatalf("exact: %v", err)
	}
	if err := ValidateMessage(strings.Repeat("z", MaxMessageLen+1)); err == nil {
		t.Fatal("oversize expected")
	}
}

func TestConstants_Positive(t *testing.T) {
	if MaxNameLen <= 0 || MaxMessageLen <= 0 || MaxPubKeyLen <= 0 {
		t.Fatal("constants must be positive")
	}
	if MaxMessageLen != 16384 {
		t.Fatalf("MaxMessageLen SoT changed: %d", MaxMessageLen)
	}
}
