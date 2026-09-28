package main

import (
	"strings"
	"testing"

	"github.com/unkillable-messenger/vpn"
)

func TestD5_NetstackLinked(t *testing.T) {
	name, err := startNetstack()
	if err != nil {
		t.Fatal(err)
	}
	if name != "gvisor-netstack" {
		t.Fatal(name)
	}
}

func TestD6_ExecutorAssigned(t *testing.T) {
	ks := vpn.NewKillSwitch("Proxi")
	if ks.Executor != nil {
		t.Fatal("executor must start nil")
	}
	BindKillSwitch(ks)
	if ks.Executor == nil {
		t.Fatal("executor still nil")
	}
}

func TestD8_ServiceName(t *testing.T) {
	if helperServiceName != "Proxi04VpnHelper" {
		t.Fatal(helperServiceName)
	}
	err := probeSCM()
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "access") && !strings.Contains(err.Error(), "denied") {
		t.Log(err)
	}
}
