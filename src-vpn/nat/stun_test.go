package nat

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"
)

func TestDefaultSTUNServers(t *testing.T) {
	t.Parallel()

	if len(DefaultSTUNServers) == 0 {
		t.Fatal("DefaultSTUNServers is empty")
	}

	for i, srv := range DefaultSTUNServers {
		if srv == "" {
			t.Errorf("server [%d] is empty", i)
			continue
		}
		if !strings.Contains(srv, ":") {
			t.Errorf("server [%d] %q missing port", i, srv)
		}
	}

	t.Logf("STUN servers: %v", DefaultSTUNServers)
}

func TestGetLocalIP(t *testing.T) {
	t.Parallel()

	ip := getLocalIP()
	if ip == "" {
		t.Fatal("getLocalIP returned empty string")
	}
	t.Logf("getLocalIP = %s", ip)
}

func TestGetTURNConfig(t *testing.T) {
	t.Parallel()

	configs := GetTURNConfig()
	if len(configs) == 0 {
		t.Fatal("GetTURNConfig returned empty slice")
	}

	for i, cfg := range configs {
		urls, ok := cfg["urls"]
		if !ok {
			t.Errorf("config [%d] missing 'urls' key", i)
			continue
		}
		urlList, ok := urls.([]string)
		if !ok {
			t.Errorf("config [%d] 'urls' is not []string", i)
			continue
		}
		if len(urlList) == 0 {
			t.Errorf("config [%d] has empty URL list", i)
		}
		for j, u := range urlList {
			if !strings.HasPrefix(u, "turn:") {
				t.Errorf("config [%d] url [%d] = %q, expected 'turn:' prefix", i, j, u)
			}
		}
	}
}

// buildSTUNBindingResponse builds a valid STUN Binding Response with XOR-MAPPED-ADDRESS.
func buildSTUNBindingResponse(t *testing.T, xorIP net.IP, xorPort uint16) []byte {
	t.Helper()
	// Attribute: XOR-MAPPED-ADDRESS (0x0020)
	// Format: Reserved(1) + Family(1, 0x01=IPv4) + XOR-Port(2) + XOR-IP(4)
	attr := make([]byte, 8)
	attr[1] = 0x01 // IPv4
	binary.BigEndian.PutUint16(attr[2:], xorPort)
	copy(attr[4:], xorIP.To4())

	// STUN header: Type=0x0101, Length=len(attr), MagicCookie=0x2112A442
	msgLen := uint16(len(attr))
	resp := make([]byte, 20+msgLen)
	binary.BigEndian.PutUint16(resp[0:], 0x0101)   // Binding Response
	binary.BigEndian.PutUint16(resp[2:], msgLen)     // Length
	binary.BigEndian.PutUint32(resp[4:], 0x2112A442) // Magic Cookie
	// Transaction ID (12 bytes)
	for i := 8; i < 20; i++ {
		resp[i] = byte(i * 17)
	}
	copy(resp[20:], attr)
	return resp
}

// buildSTUNBindingResponseMappedAddr builds a STUN response with MAPPED-ADDRESS (0x0001).
func buildSTUNBindingResponseMappedAddr(t *testing.T, ip net.IP, port uint16) []byte {
	t.Helper()
	attr := make([]byte, 8)
	attr[1] = 0x01 // IPv4
	binary.BigEndian.PutUint16(attr[2:], port)
	copy(attr[4:], ip.To4())

	msgLen := uint16(len(attr))
	resp := make([]byte, 20+msgLen)
	binary.BigEndian.PutUint16(resp[0:], 0x0101)
	binary.BigEndian.PutUint16(resp[2:], msgLen)
	binary.BigEndian.PutUint32(resp[4:], 0x2112A442)
	for i := 8; i < 20; i++ {
		resp[i] = byte(i * 17)
	}
	// Attribute header
	binary.BigEndian.PutUint16(resp[20:], 0x0001) // MAPPED-ADDRESS
	binary.BigEndian.PutUint16(resp[22:], 4+4)    // attr length = 8
	copy(resp[24:], attr)
	return resp
}

func startSTUNServer(t *testing.T, response []byte) (*net.UDPConn, string) {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		buf := make([]byte, 576)
		for {
			n, remote, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			// Verify it looks like a STUN binding request
			if n >= 20 && binary.BigEndian.Uint16(buf[0:]) == 0x0001 {
				// Copy transaction ID from request into response
				if len(response) >= 20 {
					copy(response[4:20], buf[4:20])
				}
				conn.WriteToUDP(response, remote)
			}
		}
	}()

	return conn, conn.LocalAddr().String()
}

func TestDiscoverPublicAddr_XORMappedAddr(t *testing.T) {
	// Build a response with XOR-MAPPED-ADDRESS for 203.0.113.1:12345
	publicIP := net.ParseIP("203.0.113.1")
	publicPort := uint16(12345)
	magic := uint32(0x2112A442)

	// XOR the IP and port
	xorIP := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		xorIP[i] = publicIP.To4()[i] ^ byte(magic>>((3-i)*8))
	}
	xorPort := publicPort ^ 0x2112

	// Build attribute bytes
	attrData := make([]byte, 8)
	attrData[1] = 0x01 // IPv4
	binary.BigEndian.PutUint16(attrData[2:], xorPort)
	copy(attrData[4:], xorIP)

	// Build full STUN message with attribute header
	msgLen := uint16(4 + len(attrData)) // 4 for attr type+length, 8 for data
	resp := make([]byte, 20+msgLen)
	binary.BigEndian.PutUint16(resp[0:], 0x0101)
	binary.BigEndian.PutUint16(resp[2:], msgLen)
	binary.BigEndian.PutUint32(resp[4:], 0x2112A442)
	for i := 8; i < 20; i++ {
		resp[i] = byte(i * 17)
	}
	// Attribute header
	binary.BigEndian.PutUint16(resp[20:], 0x0020) // XOR-MAPPED-ADDRESS
	binary.BigEndian.PutUint16(resp[22:], uint16(len(attrData)))
	copy(resp[24:], attrData)

	conn, addr := startSTUNServer(t, resp)
	defer conn.Close()

	result, err := DiscoverPublicAddr(addr)
	if err != nil {
		t.Fatalf("DiscoverPublicAddr failed: %v", err)
	}
	if result.PublicIP != "203.0.113.1" {
		t.Errorf("PublicIP = %q, want %q", result.PublicIP, "203.0.113.1")
	}
	if result.PublicPort != 12345 {
		t.Errorf("PublicPort = %d, want %d", result.PublicPort, 12345)
	}
	if result.NATType != "unknown" {
		t.Errorf("NATType = %q, want %q", result.NATType, "unknown")
	}
}

func TestDiscoverPublicAddr_MappedAddr(t *testing.T) {
	// Build a response with MAPPED-ADDRESS (fallback) for 198.51.100.50:54321
	publicIP := net.ParseIP("198.51.100.50")
	publicPort := uint16(54321)

	attrData := make([]byte, 8)
	attrData[1] = 0x01 // IPv4
	binary.BigEndian.PutUint16(attrData[2:], publicPort)
	copy(attrData[4:], publicIP.To4())

	msgLen := uint16(4 + len(attrData))
	resp := make([]byte, 20+msgLen)
	binary.BigEndian.PutUint16(resp[0:], 0x0101)
	binary.BigEndian.PutUint16(resp[2:], msgLen)
	binary.BigEndian.PutUint32(resp[4:], 0x2112A442)
	for i := 8; i < 20; i++ {
		resp[i] = byte(i * 17)
	}
	binary.BigEndian.PutUint16(resp[20:], 0x0001) // MAPPED-ADDRESS
	binary.BigEndian.PutUint16(resp[22:], uint16(len(attrData)))
	copy(resp[24:], attrData)

	conn, addr := startSTUNServer(t, resp)
	defer conn.Close()

	result, err := DiscoverPublicAddr(addr)
	if err != nil {
		t.Fatalf("DiscoverPublicAddr failed: %v", err)
	}
	if result.PublicIP != "198.51.100.50" {
		t.Errorf("PublicIP = %q, want %q", result.PublicIP, "198.51.100.50")
	}
	if result.PublicPort != 54321 {
		t.Errorf("PublicPort = %d, want %d", result.PublicPort, 54321)
	}
}

func TestDiscoverPublicAddr_NoMappedAddr(t *testing.T) {
	// Build a response with no mapped address attributes
	msgLen := uint16(0)
	resp := make([]byte, 20+msgLen)
	binary.BigEndian.PutUint16(resp[0:], 0x0101)
	binary.BigEndian.PutUint16(resp[2:], msgLen)
	binary.BigEndian.PutUint32(resp[4:], 0x2112A442)
	for i := 8; i < 20; i++ {
		resp[i] = byte(i * 17)
	}

	conn, addr := startSTUNServer(t, resp)
	defer conn.Close()

	_, err := DiscoverPublicAddr(addr)
	if err == nil {
		t.Fatal("expected error for response with no mapped address")
	}
	if !strings.Contains(err.Error(), "no mapped address") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDiscoverPublicAddr_InvalidServer(t *testing.T) {
	_, err := DiscoverPublicAddr("invalid-host-that-does-not-exist:99999")
	if err == nil {
		t.Fatal("expected error for invalid server")
	}
}

func TestDiscoverPublicAddr_EmptyServer(t *testing.T) {
	// Empty server should use default — this will try to connect to Google STUN.
	// It may fail in some environments, so we just ensure it doesn't panic.
	_, _ = DiscoverPublicAddr("")
}

func TestDiscoverPublicAddr_WrongResponseType(t *testing.T) {
	// Build a response with wrong message type
	resp := make([]byte, 20)
	binary.BigEndian.PutUint16(resp[0:], 0x0111) // Wrong type
	binary.BigEndian.PutUint16(resp[2:], 0)
	binary.BigEndian.PutUint32(resp[4:], 0x2112A442)
	for i := 8; i < 20; i++ {
		resp[i] = byte(i * 17)
	}

	conn, addr := startSTUNServer(t, resp)
	defer conn.Close()

	_, err := DiscoverPublicAddr(addr)
	if err == nil {
		t.Fatal("expected error for wrong response type")
	}
	if !strings.Contains(err.Error(), "unexpected STUN response type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSTUNBindingRequestFormat(t *testing.T) {
	// Verify that the STUN binding request bytes are well-formed
	// by starting a server that inspects the received packet.
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	received := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 576)
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			close(received)
			return
		}
		received <- buf[:n]
	}()

	// Send request
	srvAddr := conn.LocalAddr().String()
	udpAddr, _ := net.ResolveUDPAddr("udp", srvAddr)
	clientConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.Fatal(err)
	}

	// Build request exactly as DiscoverPublicAddr does
	req := make([]byte, 20)
	binary.BigEndian.PutUint16(req[0:], 0x0001)   // Type
	binary.BigEndian.PutUint16(req[2:], 0x0000)   // Length
	binary.BigEndian.PutUint32(req[4:], 0x2112A442) // Magic Cookie
	for i := 8; i < 20; i++ {
		req[i] = byte(i * 17)
	}
	clientConn.Write(req)
	clientConn.Close()

	data := <-received
	if data == nil {
		t.Fatal("no data received")
	}
	if len(data) != 20 {
		t.Fatalf("request length = %d, want 20", len(data))
	}
	if msgType := binary.BigEndian.Uint16(data[0:]); msgType != 0x0001 {
		t.Errorf("message type = 0x%04x, want 0x0001", msgType)
	}
	if msgLen := binary.BigEndian.Uint16(data[2:]); msgLen != 0x0000 {
		t.Errorf("message length = %d, want 0", msgLen)
	}
	if cookie := binary.BigEndian.Uint32(data[4:]); cookie != 0x2112A442 {
		t.Errorf("magic cookie = 0x%08x, want 0x2112A442", cookie)
	}
}

func TestSTUNResult_Fields(t *testing.T) {
	r := &STUNResult{
		PublicIP:   "1.2.3.4",
		PublicPort: 8080,
		NATType:    "symmetric",
	}
	if r.PublicIP != "1.2.3.4" {
		t.Errorf("PublicIP = %q", r.PublicIP)
	}
	if r.PublicPort != 8080 {
		t.Errorf("PublicPort = %d", r.PublicPort)
	}
	if r.NATType != "symmetric" {
		t.Errorf("NATType = %q", r.NATType)
	}
}

func TestTURNRelayConfig(t *testing.T) {
	cfg := TURNRelayConfig{
		Server:     "turn:example.com:3478",
		Username:   "user",
		Credential: "pass",
	}
	if cfg.Server != "turn:example.com:3478" {
		t.Errorf("Server = %q", cfg.Server)
	}
	if cfg.Username != "user" {
		t.Errorf("Username = %q", cfg.Username)
	}
	if cfg.Credential != "pass" {
		t.Errorf("Credential = %q", cfg.Credential)
	}
}

func TestDiscoverPublicAddr_TruncatedResponse(t *testing.T) {
	// Server sends response where msgLen claims more data than actually sent
	msgLen := uint16(100) // Claims 100 bytes of attributes, but we only send header
	resp := make([]byte, 20)
	binary.BigEndian.PutUint16(resp[0:], 0x0101)
	binary.BigEndian.PutUint16(resp[2:], msgLen)
	binary.BigEndian.PutUint32(resp[4:], 0x2112A442)
	for i := 8; i < 20; i++ {
		resp[i] = byte(i * 17)
	}

	conn, addr := startSTUNServer(t, resp)
	defer conn.Close()

	_, err := DiscoverPublicAddr(addr)
	if err == nil {
		t.Fatal("expected error for truncated response")
	}
}

func TestDiscoverPublicAddr_MultipleAttributes(t *testing.T) {
	// Response with XOR-MAPPED-ADDRESS + another unknown attribute
	publicIP := net.ParseIP("10.20.30.40")
	publicPort := uint16(5555)
	magic := uint32(0x2112A442)

	xorIP := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		xorIP[i] = publicIP.To4()[i] ^ byte(magic>>((3-i)*8))
	}
	xorPort := publicPort ^ 0x2112

	// First attribute: XOR-MAPPED-ADDRESS
	xorAttr := make([]byte, 4+8) // 4 header + 8 data
	binary.BigEndian.PutUint16(xorAttr[0:], 0x0020)
	binary.BigEndian.PutUint16(xorAttr[2:], 8)
	xorAttr[5] = 0x01 // IPv4
	binary.BigEndian.PutUint16(xorAttr[6:], xorPort)
	copy(xorAttr[8:], xorIP)

	// Second attribute: SOFTWARE (0x8022) with padding
	software := []byte("test-server")
	swAttrLen := uint16(len(software))
	paddedLen := swAttrLen
	if paddedLen%4 != 0 {
		paddedLen += 4 - paddedLen%4
	}
	swAttr := make([]byte, 4+paddedLen)
	binary.BigEndian.PutUint16(swAttr[0:], 0x8022)
	binary.BigEndian.PutUint16(swAttr[2:], swAttrLen)
	copy(swAttr[4:], software)

	totalAttrs := append(xorAttr, swAttr...)
	msgLen := uint16(len(totalAttrs))
	resp := make([]byte, 20+msgLen)
	binary.BigEndian.PutUint16(resp[0:], 0x0101)
	binary.BigEndian.PutUint16(resp[2:], msgLen)
	binary.BigEndian.PutUint32(resp[4:], 0x2112A442)
	for i := 8; i < 20; i++ {
		resp[i] = byte(i * 17)
	}
	copy(resp[20:], totalAttrs)

	conn, addr := startSTUNServer(t, resp)
	defer conn.Close()

	result, err := DiscoverPublicAddr(addr)
	if err != nil {
		t.Fatalf("DiscoverPublicAddr failed: %v", err)
	}
	if result.PublicIP != "10.20.30.40" {
		t.Errorf("PublicIP = %q, want %q", result.PublicIP, "10.20.30.40")
	}
	if result.PublicPort != 5555 {
		t.Errorf("PublicPort = %d, want %d", result.PublicPort, 5555)
	}
}
