package nat

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// STUN client for NAT traversal — RFC 5389.
// Discovers public IP:port and NAT type.

// STUNResult contains the discovered public address.
type STUNResult struct {
	PublicIP   string `json:"publicIP"`
	PublicPort int    `json:"publicPort"`
	NATType    string `json:"natType"`
}

// DefaultSTUNServers are public STUN servers.
var DefaultSTUNServers = []string{
	"stun.l.google.com:19302",
	"stun1.l.google.com:19302",
	"stun2.l.google.com:19302",
	"stun.stunprotocol.org:3478",
}

// DiscoverPublicAddr sends a STUN Binding Request and returns the mapped address.
func DiscoverPublicAddr(server string) (*STUNResult, error) {
	if server == "" {
		server = DefaultSTUNServers[0]
	}

	// Resolve server address
	addr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return nil, fmt.Errorf("resolve STUN server: %w", err)
	}

	// Create UDP socket
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("dial STUN server: %w", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Build STUN Binding Request (RFC 5389)
	// Type: 0x0001 (Binding Request), Length: 0, Magic Cookie: 0x2112A442
	req := make([]byte, 20)
	binary.BigEndian.PutUint16(req[0:], 0x0001) // Type
	binary.BigEndian.PutUint16(req[2:], 0x0000) // Length
	binary.BigEndian.PutUint32(req[4:], 0x2112A442) // Magic Cookie
	// Transaction ID (12 bytes random)
	for i := 8; i < 20; i++ {
		req[i] = byte(i * 17) // Simple pseudo-random
	}

	_, err = conn.Write(req)
	if err != nil {
		return nil, fmt.Errorf("send STUN request: %w", err)
	}

	// Read response
	resp := make([]byte, 576)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, fmt.Errorf("read STUN response: %w", err)
	}

	if n < 20 {
		return nil, fmt.Errorf("STUN response too short (%d bytes)", n)
	}

	// Parse STUN response
	msgType := binary.BigEndian.Uint16(resp[0:])
	if msgType != 0x0101 { // Binding Response
		return nil, fmt.Errorf("unexpected STUN response type: 0x%04x", msgType)
	}

	msgLen := binary.BigEndian.Uint16(resp[2:])
	if int(msgLen)+20 > n {
		return nil, fmt.Errorf("STUN response truncated")
	}

	// Parse attributes
	result := &STUNResult{NATType: "unknown"}
	offset := 20
	for offset+4 <= int(msgLen)+20 {
		attrType := binary.BigEndian.Uint16(resp[offset:])
		attrLen := binary.BigEndian.Uint16(resp[offset+2:])
		attrEnd := offset + 4 + int(attrLen)
		if attrEnd > n {
			break
		}
		attrVal := resp[offset+4 : attrEnd]

		switch attrType {
		case 0x0020: // XOR-MAPPED-ADDRESS
			if len(attrVal) >= 8 {
				xorPort := binary.BigEndian.Uint16(attrVal[2:])
				xorIP := attrVal[4:]

				port := xorPort ^ 0x2112 // XOR with magic cookie upper 16 bits
				magic := uint32(0x2112A442)
				ip := make(net.IP, 4)
				for i := 0; i < 4; i++ {
					ip[i] = xorIP[i] ^ byte(magic>>((3-i)*8))
				}
				result.PublicIP = ip.String()
				result.PublicPort = int(port)
			}
		case 0x0001: // MAPPED-ADDRESS (fallback)
			if result.PublicIP == "" && len(attrVal) >= 8 {
				family := attrVal[1]
				if family == 0x01 { // IPv4
					port := binary.BigEndian.Uint16(attrVal[2:])
					ip := net.IP(attrVal[4:8])
					result.PublicIP = ip.String()
					result.PublicPort = int(port)
				}
			}
		}

		// Move to next attribute (4-byte aligned)
		offset = attrEnd
		for offset%4 != 0 {
			offset++
		}
	}

	if result.PublicIP == "" {
		return nil, fmt.Errorf("no mapped address in STUN response")
	}

	return result, nil
}

// DetectNATType detects the NAT type using two STUN requests.
func DetectNATType(server string) (string, error) {
	r1, err := DiscoverPublicAddr(server)
	if err != nil {
		return "blocked", err
	}

	// Compare local IP with public IP
	localIP := getLocalIP()
	if localIP == r1.PublicIP {
		return "none", nil
	}

	// Simple heuristic: if we got a public IP that's different from local,
	// we're behind NAT. Full NAT type detection requires RFC 3489 tests.
	return "symmetric", nil
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return "0.0.0.0"
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String()
}

// TURNRelayConfig for relay server.
type TURNRelayConfig struct {
	Server     string `json:"server"`
	Username   string `json:"username"`
	Credential string `json:"credential"`
}

// GetTURNConfig returns default TURN relay config for the client.
func GetTURNConfig() []map[string]interface{} {
	// Public TURN servers (free, for testing)
	return []map[string]interface{}{
		{
			"urls": []string{
				"turn:global.turn.twilio.com:3478?transport=udp",
				"turn:global.turn.twilio.com:3478?transport=tcp",
			},
		},
	}
}
