package main

import (
	"fmt"
	"net"
	"time"
)

func isTunDNS(pkt []byte) bool {
	if len(pkt) < 28 || pkt[0]>>4 != 4 || pkt[9] != 17 {
		return false
	}
	ihl := int(pkt[0]&0x0f) * 4
	if ihl < 20 || len(pkt) < ihl+8 {
		return false
	}
	dport := int(pkt[ihl+2])<<8 | int(pkt[ihl+3])
	if dport != 53 {
		return false
	}
	dst := net.IPv4(pkt[16], pkt[17], pkt[18], pkt[19])
	return dst.Equal(net.ParseIP("10.7.0.1"))
}

func forwardDNS(payload []byte, upstream string) ([]byte, error) {
	conn, err := net.Dial("udp", upstream)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err = conn.Write(payload); err != nil {
		return nil, err
	}
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func dnsReplyPacket(req, payload []byte) ([]byte, error) {
	if !isTunDNS(req) {
		return nil, fmt.Errorf("not tun dns")
	}
	ihl := int(req[0]&0x0f) * 4
	sport := int(req[ihl])<<8 | int(req[ihl+1])
	total := ihl + 8 + len(payload)
	out := make([]byte, total)
	copy(out, req[:ihl])
	out[2] = byte(total >> 8)
	out[3] = byte(total)
	out[8] = 64
	out[10], out[11] = 0, 0
	copy(out[12:16], req[16:20])
	copy(out[16:20], req[12:16])
	sum := ipChecksum(out[:ihl])
	out[10] = byte(sum >> 8)
	out[11] = byte(sum)
	out[ihl] = 0
	out[ihl+1] = 53
	out[ihl+2] = byte(sport >> 8)
	out[ihl+3] = byte(sport)
	ulen := 8 + len(payload)
	out[ihl+4] = byte(ulen >> 8)
	out[ihl+5] = byte(ulen)
	out[ihl+6], out[ihl+7] = 0, 0
	copy(out[ihl+8:], payload)
	return out, nil
}

func ipChecksum(h []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(h); i += 2 {
		sum += uint32(h[i])<<8 | uint32(h[i+1])
	}
	for sum > 0xffff {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	return ^uint16(sum)
}

func relayDNSPacket(pkt []byte, upstream string) ([]byte, error) {
	if !isTunDNS(pkt) {
		return nil, nil
	}
	ihl := int(pkt[0]&0x0f) * 4
	payload, err := forwardDNS(pkt[ihl+8:], upstream)
	if err != nil {
		return nil, err
	}
	return dnsReplyPacket(pkt, payload)
}
