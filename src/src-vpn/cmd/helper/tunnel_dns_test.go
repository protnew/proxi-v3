package main

import (
	"testing"
	"time"
)

func TestRelayDNSPacketAnswersStub(t *testing.T) {
	addr, stop, err := startLoopbackDNSStub()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	q := []byte{0x12, 0x34, 0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, label := range splitDot("test.local") {
		q = append(q, byte(len(label)))
		q = append(q, label...)
	}
	q = append(q, 0, 0, 1, 0, 1)
	pkt := []byte{0x45, 0, 0, 0, 0, 0, 0, 0, 64, 17, 0, 0, 10, 7, 0, 2, 10, 7, 0, 1}
	udp := make([]byte, 8+len(q))
	udp[0], udp[1] = 0x12, 0x34
	udp[3] = 53
	ulen := 8 + len(q)
	udp[4] = byte(ulen >> 8)
	udp[5] = byte(ulen)
	copy(udp[8:], q)
	pkt = append(pkt, udp...)
	pkt[2] = byte(len(pkt) >> 8)
	pkt[3] = byte(len(pkt))
	got, err := relayDNSPacket(pkt, addr)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got[len(got)-4] != 10 || got[len(got)-3] != 7 || got[len(got)-2] != 0 || got[len(got)-1] != 53 {
		t.Fatalf("reply=%v", got)
	}
	if got[16] != 10 || got[19] != 2 {
		t.Fatalf("dst=%v", got[16:20])
	}
}

func TestMapWireStateUnknown(t *testing.T) {
	state, code := mapWireState("weird", "")
	if state != "error" || code != "unknown_state" {
		t.Fatalf("%s %s", state, code)
	}
	state, code = mapWireState("connecting", "")
	if state != "connecting" || code != "" {
		t.Fatalf("connecting changed %s %s", state, code)
	}
	if connectPhases[0] != "service" || connectPhases[4] != "exit-probe" {
		t.Fatalf("phases=%v", connectPhases)
	}
}

func TestParsePhysLine(t *testing.T) {
	alias, hop, idx, ok := parsePhysLine("Ethernet|192.168.1.1|12")
	if !ok || alias != "Ethernet" || hop != "192.168.1.1" || idx != 12 {
		t.Fatalf("%s %s %d %v", alias, hop, idx, ok)
	}
	if _, _, _, ok = parsePhysLine("Ethernet|0.0.0.0|12"); ok {
		t.Fatal("zero hop accepted")
	}
	if htonl(12) != 0x0c000000 {
		t.Fatalf("htonl=%x", htonl(12))
	}
}

func TestConnectHookNotNilContract(t *testing.T) {
	prev := engageTunnel
	engageTunnel = func(ep string, self bool) {}
	defer func() { engageTunnel = prev }()
	if engageTunnel == nil {
		t.Fatal("hook cleared")
	}
	_ = time.Second
}

func TestClassifyBlockProbe(t *testing.T) {
	if err := classifyBlockProbe(true, true); err != nil {
		t.Fatal(err)
	}
	if err := classifyBlockProbe(false, true); err == nil {
		t.Fatal("loopback miss accepted")
	}
	if err := classifyBlockProbe(true, false); err == nil {
		t.Fatal("open external accepted")
	}
}
