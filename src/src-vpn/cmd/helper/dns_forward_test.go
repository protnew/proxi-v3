package main

import (
	"testing"
	"time"
)

func TestDNSStubAnswersTestLocal(t *testing.T) {
	addr, stop, err := startLoopbackDNSStub()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	ip, err := queryDNS(addr, "test.local", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.7.0.53" {
		t.Fatalf("ip=%s", ip)
	}
}
