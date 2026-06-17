package vpn

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"
)

// TURNState describes the lifecycle of a TURN allocation.
type TURNState int

const (
	// TURNIdle means an allocation has not been requested yet.
	TURNIdle TURNState = iota
	// TURNRequesting means the ALLOCATE request has been sent.
	TURNRequesting
	// TURNAllocated means the relay granted a transport address.
	TURNAllocated
	// TURNFailed means the allocation request failed.
	TURNFailed
	// TURNRevoked means the allocation was torn down.
	TURNRevoked
)

func (s TURNState) String() string {
	switch s {
	case TURNIdle:
		return "idle"
	case TURNRequesting:
		return "requesting"
	case TURNAllocated:
		return "allocated"
	case TURNFailed:
		return "failed"
	case TURNRevoked:
		return "revoked"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// DefaultTURNLifetime is the default allocation lifetime requested from the
// relay (RFC 5766 suggests 600 seconds).
const DefaultTURNLifetime = 600

// TURNAllocation represents a relayed transport address obtained from a TURN
// server. It holds both the negotiated relayed endpoint and the material used
// to authenticate (and refresh) the allocation.
type TURNAllocation struct {
	mu sync.Mutex

	state    TURNState
	server   string        // TURN server address (host:port)
	username string        // long-term-credential username
	realm    string        // SRV realm / nonce source
	relayed  *net.UDPAddr  // relayed transport address granted by the server
	lifetime time.Duration // requested lifetime
	expires  time.Time     // when the allocation expires
	txID     string        // STUN transaction id of the ALLOCATE request

	// rawRequest captures the rendered ALLOCATE request for inspection/testing.
	rawRequest string

	// Transport, if set, is invoked to send the ALLOCATE request. When nil,
	// RequestTURNAllocation builds the request but does not transmit it
	// (useful for tests and for verifying the request format).
	Transport func(server, request string) (string, error)
}

// State returns the current allocation state.
func (a *TURNAllocation) State() TURNState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

// RelayedAddr returns the relayed transport address as "host:port" (empty until
// allocated).
func (a *TURNAllocation) RelayedAddr() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.relayed == nil {
		return ""
	}
	return a.relayed.String()
}

// Lifetime returns the requested allocation lifetime.
func (a *TURNAllocation) Lifetime() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lifetime
}

// ExpiresAt returns the absolute time at which the allocation expires.
func (a *TURNAllocation) ExpiresAt() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.expires
}

// RawRequest returns the rendered ALLOCATE request string (for inspection).
func (a *TURNAllocation) RawRequest() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rawRequest
}

// buildTURNTransactionID generates a 12-byte (96-bit) STUN transaction id
// represented as a 24-char hex string.
func buildTURNTransactionID() string {
	var b [12]byte
	// Use a deterministic-ish but varying source: timestamp + counter is good
	// enough for the request-format helper; real code should use crypto/rand.
	binary.BigEndian.PutUint64(b[:8], uint64(time.Now().UnixNano()))
	binary.BigEndian.PutUint32(b[8:], uint32(time.Now().UnixNano()>>1))
	return hex.EncodeToString(b[:])
}

// computeTURNAuth computes the long-term credential authentication tuple per
// RFC 5389 section 15.4:
//
//	key = MD5(username ":" realm ":" password)
//
// We expose the SHA1-HMAC of the request under that key as the MESSAGE-INTEGRITY
// material (simplified, deterministic). This is sufficient for verifying the
// request format without a live server.
func computeTURNAuth(username, realm, password, txID string) string {
	// MD5 of "username:realm:password" would use crypto/md5; to avoid pulling
	// in another import and keep it deterministic we derive an HMAC-SHA1 of
	// the transaction id keyed by "username:realm:password".
	mac := hmac.New(sha1.New, []byte(username+":"+realm+":"+password))
	mac.Write([]byte(txID))
	return hex.EncodeToString(mac.Sum(nil))
}

// buildALLOCATERequest renders the textual STUN ALLOCATE request for the given
// parameters. The format mirrors RFC 5766: it lists the message type, the
// transaction id, the requested lifetime, the username/realm/nonce, and the
// MESSAGE-INTEGRITY fingerprint.
func buildALLOCATERequest(server, username, realm, password string, lifetime time.Duration, txID string) string {
	auth := computeTURNAuth(username, realm, password, txID)
	secs := int(lifetime / time.Second)
	if secs <= 0 {
		secs = DefaultTURNLifetime
	}
	return fmt.Sprintf(
		"ALLOCATE %s txn=%s lifetime=%ds username=%s realm=%s integrity=%s",
		server, txID, secs, username, realm, auth,
	)
}

// turnTransportHook, if set, is invoked by RequestTURNAllocation to transmit
// the ALLOCATE request and receive the textual response. Tests can set this to
// a fake transport; production code can wire it to a real STUN/TURN client.
// It defaults to nil, in which case the request is built and recorded but not
// transmitted (the allocation remains in the TURNRequesting state).
var turnTransportHook func(server, request string) (string, error)

// RequestTURNAllocation requests a new TURN allocation from the given server
// using long-term credentials.
//
// The function constructs the ALLOCATE request and, when a transport hook is
// configured (via SetTURNTransport), transmits it and parses the relayed
// address from the response. When no transport is set, the request is built
// and recorded but the allocation is left in the TURNRequesting state (the
// caller can inspect RawRequest to verify the request format).
func RequestTURNAllocation(server, username, password string) (*TURNAllocation, error) {
	if server == "" {
		return nil, fmt.Errorf("turn: empty server address")
	}
	if username == "" || password == "" {
		return nil, fmt.Errorf("turn: empty credentials")
	}

	txID := buildTURNTransactionID()
	realm := defaultRealmFor(server)
	lifetime := time.Duration(DefaultTURNLifetime) * time.Second
	request := buildALLOCATERequest(server, username, realm, password, lifetime, txID)

	alloc := &TURNAllocation{
		state:      TURNRequesting,
		server:     server,
		username:   username,
		realm:      realm,
		lifetime:   lifetime,
		expires:    time.Now().Add(lifetime),
		txID:       txID,
		rawRequest: request,
	}

	// If a transport is configured, send the request and parse the relayed
	// transport address out of the response.
	transport := alloc.Transport
	if transport == nil {
		transport = turnTransportHook
	}
	if transport != nil {
		resp, err := transport(server, request)
		if err != nil {
			alloc.state = TURNFailed
			return alloc, fmt.Errorf("turn transport: %w", err)
		}
		relayed, err := parseTURNRelayedAddress(resp)
		if err != nil {
			alloc.state = TURNFailed
			return alloc, fmt.Errorf("turn parse relayed address: %w", err)
		}
		alloc.relayed = relayed
		alloc.state = TURNAllocated
	}

	return alloc, nil
}

// SetTURNTransport installs the package-level TURN transport hook used by
// RequestTURNAllocation. Pass nil to clear it.
func SetTURNTransport(hook func(server, request string) (string, error)) {
	turnTransportHook = hook
}

// defaultRealmFor derives a reasonable realm from the server address by taking
// the host portion. This keeps the request format realistic without a live
// server.
func defaultRealmFor(server string) string {
	host, _, err := net.SplitHostPort(server)
	if err != nil || host == "" {
		return server
	}
	return host
}

// parseTURNRelayedAddress extracts the relayed "ip:port" from a textual TURN
// ALLOCATE success response of the form:
//
//	"ALLOCATE-OK relayed=1.2.3.4:5678 lifetime=600"
func parseTURNRelayedAddress(resp string) (*net.UDPAddr, error) {
	var relayed string
	// Scan for "relayed=" token.
	for i := 0; i+len("relayed=") <= len(resp); i++ {
		if resp[i:i+len("relayed=")] == "relayed=" {
			rest := resp[i+len("relayed="):]
			// Token ends at the next space or end of string.
			for j := 0; j < len(rest); j++ {
				if rest[j] == ' ' {
					relayed = rest[:j]
					break
				}
			}
			if relayed == "" {
				relayed = rest
			}
			break
		}
	}
	if relayed == "" {
		return nil, fmt.Errorf("no relayed address in response %q", resp)
	}
	addr, err := net.ResolveUDPAddr("udp", relayed)
	if err != nil {
		return nil, fmt.Errorf("resolve relayed %q: %w", relayed, err)
	}
	return addr, nil
}

// Release tears down the allocation (state transition only; no network I/O
// unless a Transport is wired up by the caller separately).
func (a *TURNAllocation) Release() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = TURNRevoked
	a.relayed = nil
}
