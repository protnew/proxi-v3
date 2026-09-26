package main

import "testing"

func TestReconcileOrphans(t *testing.T) {
	rep := reconcileOrphans(2, []string{"0.0.0.0/1 via 10.7.0.1", "8.8.8.8/32 via 1.2.3.4"}, []string{"Proxi0 10.7.0.1", "Ethernet 1.1.1.1"})
	if len(rep.Actions) != 3 || len(rep.Routes) != 1 || len(rep.DNS) != 1 {
		t.Fatalf("%+v", rep)
	}
	empty := reconcileOrphans(0, nil, nil)
	if len(empty.Actions) != 0 {
		t.Fatalf("%+v", empty)
	}
}
