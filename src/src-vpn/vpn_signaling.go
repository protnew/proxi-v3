package vpn

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// VPN Signaling via Nostr events (kind:30090).
// Architecture tables 26, 53 — Nostr is the chosen signaling protocol.
//
// Event types:
//   vpn-invite   — Alice offers to share her exit node with Bob
//   vpn-request  — Bob asks Alice for VPN access
//   vpn-accept   — accept an invite or request
//   vpn-reject   — reject an invite or request
//   vpn-cancel   — cancel an active share

const VPNEventKind = 30090

// VPNEvent is the payload of a kind:30090 Nostr event content field.
type VPNEvent struct {
	Type      string `json:"type"`       // vpn-invite, vpn-request, vpn-accept, vpn-reject, vpn-cancel
	From      string `json:"from"`       // sender npub
	To        string `json:"to"`         // recipient npub
	WTPort    int    `json:"wtPort"`     // WebTransport server port (for accept)
	WTCertHash string `json:"wtCertHash"`// WebTransport certificate hash
	WTAddr    string `json:"wtAddr"`     // WebTransport server address (host:port)
	Timestamp int64  `json:"timestamp"`
}

// VPNSignaling manages VPN-related Nostr events.
type VPNSignaling struct {
	mu          sync.Mutex
	pendingInvite map[string]*VPNEvent // eventID → event
	handlers    map[string][]func(VPNEvent) // eventType → handlers
}

// NewVPNSignaling creates a new VPN signaling manager.
func NewVPNSignaling() *VPNSignaling {
	return &VPNSignaling{
		pendingInvite: make(map[string]*VPNEvent),
		handlers:      make(map[string][]func(VPNEvent)),
	}
}

// OnVPNEvent registers a handler for a specific VPN event type.
func (vs *VPNSignaling) OnVPNEvent(eventType string, handler func(VPNEvent)) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.handlers[eventType] = append(vs.handlers[eventType], handler)
}

// CreateVPNInvite creates a vpn-invite event for offering VPN access to a friend.
func (vs *VPNSignaling) CreateVPNInvite(fromNpub, toNpub string, wtAddr, wtCertHash string) VPNEvent {
	return VPNEvent{
		Type:      "vpn-invite",
		From:      fromNpub,
		To:        toNpub,
		WTAddr:    wtAddr,
		WTCertHash: wtCertHash,
		Timestamp: time.Now().Unix(),
	}
}

// CreateVPNRequest creates a vpn-request event for asking a friend for VPN access.
func (vs *VPNSignaling) CreateVPNRequest(fromNpub, toNpub string) VPNEvent {
	return VPNEvent{
		Type:      "vpn-request",
		From:      fromNpub,
		To:        toNpub,
		Timestamp: time.Now().Unix(),
	}
}

// CreateVPNAccept creates a vpn-accept event.
func (vs *VPNSignaling) CreateVPNAccept(fromNpub, toNpub string, wtAddr, wtCertHash string) VPNEvent {
	return VPNEvent{
		Type:      "vpn-accept",
		From:      fromNpub,
		To:        toNpub,
		WTAddr:    wtAddr,
		WTCertHash: wtCertHash,
		Timestamp: time.Now().Unix(),
	}
}

// CreateVPNReject creates a vpn-reject event.
func (vs *VPNSignaling) CreateVPNReject(fromNpub, toNpub string) VPNEvent {
	return VPNEvent{
		Type:      "vpn-reject",
		From:      fromNpub,
		To:        toNpub,
		Timestamp: time.Now().Unix(),
	}
}

// HandleIncomingEvent processes an incoming VPN event (called when Nostr event received).
func (vs *VPNSignaling) HandleIncomingEvent(eventID string, event VPNEvent) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	switch event.Type {
	case "vpn-invite", "vpn-request":
		vs.pendingInvite[eventID] = &event
		log.Printf("[VPN-SIG] Received %s from %s for %s\n", event.Type, event.From, event.To)
	case "vpn-accept":
		log.Printf("[VPN-SIG] %s accepted VPN from %s. WT addr: %s\n", event.From, event.To, event.WTAddr)
	case "vpn-reject":
		log.Printf("[VPN-SIG] %s rejected VPN from %s\n", event.From, event.To)
	case "vpn-cancel":
		log.Printf("[VPN-SIG] %s cancelled VPN\n", event.From)
	}

	// Call registered handlers
	if handlers, ok := vs.handlers[event.Type]; ok {
		for _, h := range handlers {
			go h(event)
		}
	}
}

// GetPendingInvites returns all pending VPN invites/requests.
func (vs *VPNSignaling) GetPendingInvites() []VPNEvent {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	result := make([]VPNEvent, 0, len(vs.pendingInvite))
	for _, e := range vs.pendingInvite {
		result = append(result, *e)
	}
	return result
}

// SerializeVPNEvent converts a VPNEvent to JSON bytes for Nostr event content.
func SerializeVPNEvent(e VPNEvent) ([]byte, error) {
	return json.Marshal(e)
}

// DeserializeVPNEvent parses JSON bytes into a VPNEvent.
func DeserializeVPNEvent(data []byte) (VPNEvent, error) {
	var e VPNEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return e, fmt.Errorf("vpn event unmarshal: %w", err)
	}
	return e, nil
}
