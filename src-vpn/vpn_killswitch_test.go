package vpn

import (
	"strings"
	"testing"
)

func TestKillSwitch_EnableDisableCycle(t *testing.T) {
	ks := NewKillSwitch("tun0")

	if ks.IsActive() {
		t.Fatal("kill switch should be inactive initially")
	}
	if ks.State() != KillSwitchInactive {
		t.Errorf("State = %v, want %v", ks.State(), KillSwitchInactive)
	}

	// Enable -> active.
	if err := ks.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !ks.IsActive() {
		t.Fatal("kill switch should be active after Enable")
	}
	applied := ks.AppliedRules()
	if len(applied) == 0 {
		t.Fatal("expected applied rules after Enable")
	}
	// The DROP rule and the tunnel ACCEPT rules must be present.
	joined := strings.Join(applied, "\n")
	if !strings.Contains(joined, "-j DROP") {
		t.Error("applied rules should contain a DROP rule")
	}
	if !strings.Contains(joined, "tun0") {
		t.Error("applied rules should reference the tunnel interface tun0")
	}
	if !strings.Contains(joined, DefaultKillSwitchChain) {
		t.Error("applied rules should reference the kill switch chain")
	}

	// Disable -> inactive.
	if err := ks.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if ks.IsActive() {
		t.Fatal("kill switch should be inactive after Disable")
	}
	if len(ks.AppliedRules()) != 0 {
		t.Error("AppliedRules should be empty after Disable")
	}

	// The full Rules() history should contain both enable and disable rules.
	hist := ks.Rules()
	hasEnable := false
	hasDisable := false
	for _, r := range hist {
		if strings.Contains(r, "-N "+DefaultKillSwitchChain) {
			hasEnable = true
		}
		if strings.Contains(r, "-X "+DefaultKillSwitchChain) {
			hasDisable = true
		}
	}
	if !hasEnable {
		t.Error("Rules history should contain chain creation (-N)")
	}
	if !hasDisable {
		t.Error("Rules history should contain chain deletion (-X)")
	}
}

func TestKillSwitch_DoubleEnableSafe(t *testing.T) {
	ks := NewKillSwitch("tun0")

	if err := ks.Enable(); err != nil {
		t.Fatalf("first Enable: %v", err)
	}
	rulesAfterFirst := len(ks.Rules())

	// Enabling again must be a safe no-op.
	if err := ks.Enable(); err != nil {
		t.Fatalf("second Enable: %v", err)
	}
	rulesAfterSecond := len(ks.Rules())

	if rulesAfterFirst != rulesAfterSecond {
		t.Errorf("double-enable added rules: %d -> %d", rulesAfterFirst, rulesAfterSecond)
	}
	if !ks.IsActive() {
		t.Error("should still be active after double-enable")
	}
}

func TestKillSwitch_DoubleDisableSafe(t *testing.T) {
	ks := NewKillSwitch("tun0")

	if err := ks.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if err := ks.Disable(); err != nil {
		t.Fatalf("first Disable: %v", err)
	}
	rulesAfterFirst := len(ks.Rules())

	// Disabling again must be a safe no-op.
	if err := ks.Disable(); err != nil {
		t.Fatalf("second Disable: %v", err)
	}
	if len(ks.Rules()) != rulesAfterFirst {
		t.Errorf("double-disable changed rule history: %d -> %d", rulesAfterFirst, len(ks.Rules()))
	}
	if ks.IsActive() {
		t.Error("should be inactive after double-disable")
	}
}

func TestKillSwitch_AllowedInterface(t *testing.T) {
	ks := NewKillSwitch("tun0")
	ks.AllowInterface("eth1")

	if err := ks.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	joined := strings.Join(ks.AppliedRules(), "\n")
	if !strings.Contains(joined, "eth1") {
		t.Error("applied rules should reference allowed interface eth1")
	}
}

func TestKillSwitch_StateString(t *testing.T) {
	if KillSwitchActive.String() != "active" {
		t.Errorf("active string = %q", KillSwitchActive.String())
	}
	if KillSwitchInactive.String() != "inactive" {
		t.Errorf("inactive string = %q", KillSwitchInactive.String())
	}
}
