package main

import "testing"

func TestPowerEventResumeReasserts(t *testing.T) {
	got := handlePowerEvent(serviceControlPowerEvent, pbtApmResumeAutomatic)
	if !got.Reassert || !got.SignalCore {
		t.Fatalf("resume automatic: %+v", got)
	}
	got = handlePowerEvent(serviceControlPowerEvent, pbtApmResumeSuspend)
	if !got.Reassert || !got.SignalCore {
		t.Fatalf("resume suspend: %+v", got)
	}
	got = handlePowerEvent(1, pbtApmResumeAutomatic)
	if got.Reassert || got.SignalCore {
		t.Fatalf("non-power control must be idle: %+v", got)
	}
}
