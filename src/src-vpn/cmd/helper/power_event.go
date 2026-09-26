package main

const (
	serviceControlPowerEvent uint32 = 0x0000000D
	pbtApmResumeSuspend      uint32 = 0x0007
	pbtApmResumeAutomatic    uint32 = 0x0012
)

type PowerAction struct {
	Reassert   bool
	SignalCore bool
}

func handlePowerEvent(control, eventType uint32) PowerAction {
	if control != serviceControlPowerEvent {
		return PowerAction{}
	}
	if eventType == pbtApmResumeSuspend || eventType == pbtApmResumeAutomatic {
		return PowerAction{Reassert: true, SignalCore: true}
	}
	return PowerAction{}
}

func powerEvent() {
	emit("power-resume TODO re-assert routes")
}
