package main

import (
	"strings"
	"testing"
)

func TestD8_Stage1DoesNotClaimTun(t *testing.T) {
	err := stage1Only()
	if err == nil || !strings.Contains(err.Error(), WFPSublayerName) {
		t.Fatal(err)
	}
	if WFPSublayerName != "PROXI_KILLSWITCH" {
		t.Fatal(WFPSublayerName)
	}
}
