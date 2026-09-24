package vpn

// Phase-0 UDP (D3 / O7): drop datagrams and answer DNS inside the tunnel
// so queries never leave on the physical interface.
const Phase0UDPMode = "dropped"

type TunnelTelemetry struct {
	UDPMode string `json:"udp_mode"`
	Leg     string `json:"leg"`
}

func Phase0Telemetry(leg string) TunnelTelemetry {
	return TunnelTelemetry{UDPMode: Phase0UDPMode, Leg: leg}
}

// DropUDP reports that a datagram was not forwarded.
func DropUDP(payload []byte) (forward []byte, mode string) {
	return nil, Phase0UDPMode
}

// FakeDNS builds a minimal response so the tunnel does not dial :53 outside.
// Query bytes are echoed with the QR bit set; this is not a recursive resolver.
func FakeDNS(query []byte) []byte {
	if len(query) < 12 {
		return []byte{0, 0, 0x80, 0x03}
	}
	out := append([]byte(nil), query...)
	out[2] |= 0x80
	out[3] = 0x03
	return out
}
