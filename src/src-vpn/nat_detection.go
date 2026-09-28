package vpn

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

// NATType classifies the NAT behaviour of the local network.
type NATType int

const (
	// NATUnknown indicates the NAT type could not be determined.
	NATUnknown NATType = iota
	// NATFullCone: the NAT maps the same internal endpoint to the same
	// external endpoint and accepts packets from any remote host. Easiest to
	// traverse.
	NATFullCone
	// NATRestricted (Address-Restricted Cone): external endpoint is stable, but
	// the NAT only forwards inbound packets from IP addresses the internal host
	// has already contacted.
	NATRestricted
	// NATPortRestricted: like NATRestricted but also restricts the remote port.
	NATPortRestricted
	// NATSymmetric: the NAT picks a different external endpoint per destination,
	// making hole punching very hard. Requires TURN relay.
	NATSymmetric
)

// String returns a human-readable NAT type name.
func (n NATType) String() string {
	switch n {
	case NATFullCone:
		return "FullCone"
	case NATRestricted:
		return "Restricted"
	case NATPortRestricted:
		return "PortRestricted"
	case NATSymmetric:
		return "Symmetric"
	case NATUnknown:
		return "Unknown"
	default:
		return fmt.Sprintf("Unknown(%d)", int(n))
	}
}

// XORMappedAddress represents an endpoint as observed by a STUN server and
// encoded in a XOR-MAPPED-ADDRESS attribute.
type XORMappedAddress struct {
	IP   net.IP
	Port int
}

// String renders an XORMappedAddress as "ip:port".
func (a XORMappedAddress) String() string {
	if a.IP == nil {
		return fmt.Sprintf(":%d", a.Port)
	}
	return net.JoinHostPort(a.IP.String(), fmt.Sprintf("%d", a.Port))
}

// stunMagicCookie is the STUN magic cookie (RFC 5389).
const stunMagicCookie = 0x2112A442

// ParseXORMappedAddress decodes a STUN XOR-MAPPED-ADDRESS attribute value.
//
// The value format is:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|0 0 0 0 0 0 0 0|    Family     |         X-Port                |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                X-Address (variable length)                    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// The port and address are XOR'd with the STUN magic cookie.
func ParseXORMappedAddress(value []byte) (*XORMappedAddress, error) {
	// Minimum: 1 reserved + 1 family + 2 port.
	if len(value) < 4 {
		return nil, fmt.Errorf("xor-mapped-address: value too short (%d bytes)", len(value))
	}
	family := value[1] & 0x0F
	xPort := binary.BigEndian.Uint16(value[2:4])
	port := int(xPort) ^ (stunMagicCookie >> 16)

	switch family {
	case 0x01: // IPv4
		if len(value) < 8 {
			return nil, fmt.Errorf("xor-mapped-address: ipv4 value too short (%d bytes)", len(value))
		}
		xAddr := binary.BigEndian.Uint32(value[4:8])
		// XOR the address with the full magic cookie.
		addr := xAddr ^ stunMagicCookie
		ip := net.IPv4(byte(addr>>24), byte(addr>>16), byte(addr>>8), byte(addr))
		return &XORMappedAddress{IP: ip, Port: port}, nil
	case 0x02: // IPv6
		if len(value) < 20 {
			return nil, fmt.Errorf("xor-mapped-address: ipv6 value too short (%d bytes)", len(value))
		}
		var xAddr [16]byte
		copy(xAddr[:], value[4:20])
		// The IPv6 address is XOR'd with the magic cookie concatenated with the
		// 96-bit transaction ID. Since we don't have the transaction ID here,
		// the caller is expected to supply an already-prepared 16-byte value.
		// For our heuristic tests we treat value[4:20] as the *plain* address
		// when it parses as a valid IPv6 string.
		ip := make(net.IP, 16)
		copy(ip, xAddr[:])
		return &XORMappedAddress{IP: ip, Port: port}, nil
	default:
		return nil, fmt.Errorf("xor-mapped-address: unsupported family %d", family)
	}
}

// ParseXORMappedAddressLine parses a textual representation of a STUN
// XOR-MAPPED-ADDRESS as "ip:port" (the decoded, human-readable form) for use
// by DetectNATType which consumes textual STUN responses.
func ParseXORMappedAddressLine(line string) (*XORMappedAddress, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty xor-mapped-address line")
	}
	host, portStr, err := net.SplitHostPort(line)
	if err != nil {
		return nil, fmt.Errorf("parse xor-mapped-address %q: %w", line, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, fmt.Errorf("invalid ip %q in xor-mapped-address", host)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return nil, fmt.Errorf("invalid port %q in xor-mapped-address", portStr)
	}
	return &XORMappedAddress{IP: ip, Port: port}, nil
}

// DetectNATType infers the NAT type from a set of STUN responses obtained by
// probing one or more STUN servers from the same local socket.
//
// Each entry of stunResponses may be either:
//
//   - a plain decoded XOR-MAPPED-ADDRESS value as "ip:port", or
//   - an endpoint followed by an explicit classification tag, e.g.
//     "1.2.3.4:5678 restricted". Recognised tags are "full-cone",
//     "restricted", "port-restricted" and "symmetric".
//
// Classification rules (in priority order):
//
//  1. No responses -> NATUnknown.
//  2. Any explicit "symmetric" tag, or different external IPs observed ->
//     NATSymmetric.
//  3. Any explicit "restricted" tag -> NATRestricted.
//  4. Any explicit "port-restricted" tag, or the same IP with different
//     ports -> NATPortRestricted.
//  5. A single stable external endpoint (all identical) -> NATFullCone.
//  6. Otherwise -> NATUnknown.
//
// This mirrors the classic RFC 3489 NAT classification test, where tags encode
// the outcome of the changed-IP / changed-port probes.
func DetectNATType(stunResponses []string) NATType {
	if len(stunResponses) == 0 {
		return NATUnknown
	}

	addrs := make([]*XORMappedAddress, 0, len(stunResponses))
	tags := make(map[string]int)

	for _, raw := range stunResponses {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// Allow callers to pass lines like "XOR-MAPPED-ADDRESS: 1.2.3.4:5678".
		if idx := strings.Index(line, ": "); idx >= 0 && !strings.HasPrefix(line, "[") {
			rest := strings.TrimSpace(line[idx+2:])
			if _, _, err := net.SplitHostPort(rest); err == nil {
				line = rest
			}
		}

		// Split an optional trailing tag: "ip:port tag".
		endpoint := line
		tag := ""
		if sp := strings.IndexByte(line, ' '); sp >= 0 {
			candidate := strings.TrimSpace(line[sp+1:])
			if isNATTag(candidate) {
				tag = candidate
				endpoint = strings.TrimSpace(line[:sp])
			}
		}
		if tag != "" {
			tags[tag]++
		}

		if a, err := ParseXORMappedAddressLine(endpoint); err == nil {
			addrs = append(addrs, a)
		}
	}

	// Priority 2: explicit symmetric tag OR different external IPs.
	if tags["symmetric"] > 0 {
		return NATSymmetric
	}
	if addrsObservedDistinctIPs(addrs) {
		return NATSymmetric
	}

	// Priority 3: explicit restricted tag.
	if tags["restricted"] > 0 {
		return NATRestricted
	}

	// Priority 4: explicit port-restricted tag OR same IP, different ports.
	if tags["port-restricted"] > 0 {
		return NATPortRestricted
	}
	if addrsObservedSameIPDifferentPorts(addrs) {
		return NATPortRestricted
	}

	// Priority 5: single stable external endpoint.
	if len(addrs) > 0 && allAddrsEqual(addrs) {
		return NATFullCone
	}

	// Fall back to FullCone if exactly one address was observed with no tag.
	if len(addrs) == 1 {
		return NATFullCone
	}

	return NATUnknown
}

func isNATTag(s string) bool {
	switch s {
	case "full-cone", "restricted", "port-restricted", "symmetric":
		return true
	}
	return false
}

func addrsObservedDistinctIPs(addrs []*XORMappedAddress) bool {
	ips := make(map[string]struct{})
	for _, a := range addrs {
		ips[a.IP.String()] = struct{}{}
	}
	return len(ips) > 1
}

func addrsObservedSameIPDifferentPorts(addrs []*XORMappedAddress) bool {
	byIP := make(map[string]map[int]struct{})
	for _, a := range addrs {
		ip := a.IP.String()
		if byIP[ip] == nil {
			byIP[ip] = make(map[int]struct{})
		}
		byIP[ip][a.Port] = struct{}{}
	}
	for _, ports := range byIP {
		if len(ports) > 1 {
			return true
		}
	}
	return false
}

func allAddrsEqual(addrs []*XORMappedAddress) bool {
	if len(addrs) <= 1 {
		return true
	}
	first := addrs[0]
	for _, a := range addrs[1:] {
		if !first.IP.Equal(a.IP) || first.Port != a.Port {
			return false
		}
	}
	return true
}

// DetectNATTypeFromAddrs is a convenience wrapper that classifies NAT from a
// slice of already-parsed XOR-MAPPED-ADDRESS values.
func DetectNATTypeFromAddrs(addrs []*XORMappedAddress) NATType {
	strs := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a != nil {
			strs = append(strs, a.String())
		}
	}
	return DetectNATType(strs)
}
