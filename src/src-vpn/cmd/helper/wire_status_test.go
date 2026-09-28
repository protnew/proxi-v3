package main

import "testing"

func TestWireStatusShapes(t *testing.T) {
	c := wireFrom("connecting", "routes", "", "userspace", "", "1.2.3.4:443", true, false, 0, 0)
	if c.State != "connecting" || c.Phase != "routes" || c.Code != "" {
		t.Fatalf("%+v", c)
	}
	r := wireFrom("reconnecting", "", "", "", "", "", true, false, 2, 5)
	if r.Attempt != 2 || r.Max != 5 || r.Phase != "" {
		t.Fatalf("%+v", r)
	}
	e := wireFrom("error", "", "route_add_failed", "", "route_add_failed", "", false, false, 0, 0)
	if e.Code != "route_add_failed" {
		t.Fatalf("%+v", e)
	}
	ok := wireFrom("connected", "routes", "", "wt", "", "1.2.3.4:443", true, true, 0, 0)
	if ok.Leg != "wt" || !ok.SelfExit || ok.Phase != "" {
		t.Fatalf("%+v", ok)
	}
	sh := wireFrom("sharing", "", "", "", "", "", true, false, 0, 0)
	if sh.State != "sharing" || sh.Leg != "" {
		t.Fatalf("%+v", sh)
	}
	bad := wireFrom("mystery", "", "", "", "", "", false, false, 0, 0)
	if bad.State != "error" || bad.Code != "unknown_state" {
		t.Fatalf("%+v", bad)
	}
}

func TestPipeClientGoneLocks(t *testing.T) {
	state, hold := onPipeClientGone(0, true)
	if state != "locked" || !hold {
		t.Fatalf("%s %v", state, hold)
	}
	if s, h := onPipeClientGone(1, true); s != "" || h {
		t.Fatalf("client still up %s %v", s, h)
	}
	if s, h := onPipeClientGone(0, false); s != "" || h {
		t.Fatalf("idle changed %s %v", s, h)
	}
}

func TestPlanResumeReasserts(t *testing.T) {
	idle := planResume(false, "Proxi0")
	if idle.ReassertRoutes {
		t.Fatal("idle reassert")
	}
	on := planResume(true, "Proxi0")
	if !on.ReassertRoutes || !on.HealthCheck || !on.SignalCore || len(on.Actions) != 3 {
		t.Fatalf("%+v", on)
	}
}

func TestDHCPPermitScope(t *testing.T) {
	if !dhcpPermitOK("connect-v4", true, 67) || !dhcpPermitOK("connect-v6", true, 547) {
		t.Fatal("expected permits dropped")
	}
	if dhcpPermitOK("recv-v4", true, 67) || dhcpPermitOK("connect-v4", false, 67) || dhcpPermitOK("connect-v6", true, 67) {
		t.Fatal("wide dhcp accepted")
	}
}
