package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func dnsQuestionName(q []byte) (string, bool) {
	if len(q) < 12 {
		return "", false
	}
	i := 12
	var parts []byte
	for i < len(q) {
		n := int(q[i])
		if n == 0 {
			break
		}
		if n > 63 || i+1+n > len(q) {
			return "", false
		}
		if len(parts) > 0 {
			parts = append(parts, '.')
		}
		parts = append(parts, q[i+1:i+1+n]...)
		i += 1 + n
	}
	return string(parts), true
}

func dnsAnswerA(query []byte, ip net.IP) []byte {
	ip4 := ip.To4()
	if ip4 == nil || len(query) < 12 {
		return nil
	}
	resp := append([]byte(nil), query...)
	binary.BigEndian.PutUint16(resp[2:], 0x8180)
	binary.BigEndian.PutUint16(resp[6:], 1)
	resp = append(resp, 0xC0, 0x0C)
	resp = append(resp, 0, 1, 0, 1)
	resp = append(resp, 0, 0, 0, 30)
	resp = append(resp, 0, 4)
	resp = append(resp, ip4...)
	return resp
}

func startLoopbackDNSStub() (string, func(), error) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:5353")
	if err != nil {
		pc, err = net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			return "", nil, err
		}
	}
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 1500)
		for {
			_ = pc.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			n, addr, err := pc.ReadFrom(buf)
			select {
			case <-done:
				return
			default:
			}
			if err != nil {
				continue
			}
			name, ok := dnsQuestionName(buf[:n])
			if !ok || name != "test.local" {
				continue
			}
			ans := dnsAnswerA(buf[:n], net.ParseIP("10.7.0.53"))
			if ans != nil {
				_, _ = pc.WriteTo(ans, addr)
			}
		}
	}()
	stop := func() {
		close(done)
		_ = pc.Close()
	}
	return pc.LocalAddr().String(), stop, nil
}

func queryDNS(addr, name string, timeout time.Duration) (net.IP, error) {
	pc, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	defer pc.Close()
	_ = pc.SetDeadline(time.Now().Add(timeout))
	q := []byte{0x12, 0x34, 0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, label := range splitDot(name) {
		q = append(q, byte(len(label)))
		q = append(q, label...)
	}
	q = append(q, 0, 0, 1, 0, 1)
	if _, err := pc.Write(q); err != nil {
		return nil, err
	}
	buf := make([]byte, 1500)
	n, err := pc.Read(buf)
	if err != nil {
		return nil, err
	}
	if n < 12+16 {
		return nil, fmt.Errorf("short dns reply %d", n)
	}
	return net.IP(buf[n-4 : n]), nil
}

func splitDot(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '.' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}
