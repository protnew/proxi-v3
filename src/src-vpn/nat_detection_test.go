package vpn

import (
	"encoding/binary"
	"testing"
)

func TestNATDetect_EachType(t *testing.T) {
	cases := []struct {
		name   string
		input  []string
		expect NATType
	}{
		{
			name:   "no responses -> unknown",
			input:  nil,
			expect: NATUnknown,
		},
		{
			name:   "single endpoint -> full cone",
			input:  []string{"203.0.113.5:48000"},
			expect: NATFullCone,
		},
		{
			name:   "identical endpoints from multiple servers -> full cone",
			input:  []string{"203.0.113.5:48000", "203.0.113.5:48000"},
			expect: NATFullCone,
		},
		{
			name:   "explicit restricted tag",
			input:  []string{"203.0.113.5:48000 restricted"},
			expect: NATRestricted,
		},
		{
			name:   "same IP different ports -> port restricted",
			input:  []string{"203.0.113.5:48000", "203.0.113.5:48001"},
			expect: NATPortRestricted,
		},
		{
			name:   "explicit port-restricted tag",
			input:  []string{"203.0.113.5:48000 port-restricted"},
			expect: NATPortRestricted,
		},
		{
			name:   "different IPs -> symmetric",
			input:  []string{"203.0.113.5:48000", "198.51.100.7:48000"},
			expect: NATSymmetric,
		},
		{
			name:   "explicit symmetric tag",
			input:  []string{"203.0.113.5:48000 symmetric"},
			expect: NATSymmetric,
		},
		{
			name:   "attribute-prefixed line is parsed",
			input:  []string{"XOR-MAPPED-ADDRESS: 203.0.113.5:48000"},
			expect: NATFullCone,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := DetectNATType(c.input)
			if got != c.expect {
				t.Errorf("DetectNATType(%v) = %v, want %v", c.input, got, c.expect)
			}
		})
	}
}

func TestNATType_String(t *testing.T) {
	expectations := map[NATType]string{
		NATFullCone:       "FullCone",
		NATRestricted:     "Restricted",
		NATPortRestricted: "PortRestricted",
		NATSymmetric:      "Symmetric",
		NATUnknown:        "Unknown",
	}
	for typ, want := range expectations {
		if got := typ.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", typ, got, want)
		}
	}
}

func TestParseXORMappedAddressLine(t *testing.T) {
	a, err := ParseXORMappedAddressLine("203.0.113.5:48000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Port != 48000 {
		t.Errorf("port = %d, want 48000", a.Port)
	}
	if a.IP.String() != "203.0.113.5" {
		t.Errorf("ip = %s, want 203.0.113.5", a.IP.String())
	}

	// Invalid inputs must error.
	if _, err := ParseXORMappedAddressLine("not-an-endpoint"); err == nil {
		t.Error("expected error for invalid input")
	}
	if _, err := ParseXORMappedAddressLine("999.999.999.999:80"); err == nil {
		t.Error("expected error for invalid ip")
	}
}

// TestParseXORMappedAddress_Binary verifies decoding of a raw STUN
// XOR-MAPPED-ADDRESS attribute value for IPv4.
func TestParseXORMappedAddress_Binary(t *testing.T) {
	const wantIP = "203.0.113.5"
	const wantPort = 48000

	// Build the XOR'd value: family=0x01 (IPv4), port XOR (cookie>>16),
	// address XOR full cookie.
	var buf [8]byte
	buf[1] = 0x01 // family IPv4
	xPort := uint16(wantPort) ^ uint16(stunMagicCookie>>16)
	binary.BigEndian.PutUint16(buf[2:4], xPort)

	// Encode the IPv4 address.
	ipBytes := []byte{203, 0, 113, 5}
	addr32 := binary.BigEndian.Uint32(ipBytes)
	xAddr := addr32 ^ stunMagicCookie
	binary.BigEndian.PutUint32(buf[4:8], xAddr)

	a, err := ParseXORMappedAddress(buf[:])
	if err != nil {
		t.Fatalf("ParseXORMappedAddress: %v", err)
	}
	if a.Port != wantPort {
		t.Errorf("port = %d, want %d", a.Port, wantPort)
	}
	if a.IP.String() != wantIP {
		t.Errorf("ip = %s, want %s", a.IP.String(), wantIP)
	}
}

func TestParseXORMappedAddress_BinaryErrors(t *testing.T) {
	if _, err := ParseXORMappedAddress(nil); err == nil {
		t.Error("expected error for nil input")
	}
	// Unsupported family.
	bad := []byte{0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if _, err := ParseXORMappedAddress(bad); err == nil {
		t.Error("expected error for unsupported family")
	}
}
