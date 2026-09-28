package vpn

import (
	"testing"
	"time"
)

func TestVPNEventSerialize(t *testing.T) {
	original := VPNEvent{
		Type:      "vpn-invite",
		From:      "alice_npub_123",
		To:        "bob_npub_456",
		WTAddr:    "10.0.0.1:443",
		WTCertHash: "abcdef1234567890",
		Timestamp: time.Now().Unix(),
	}

	data, err := SerializeVPNEvent(original)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := DeserializeVPNEvent(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	if parsed.Type != original.Type {
		t.Errorf("type mismatch: %s != %s", parsed.Type, original.Type)
	}
	if parsed.From != original.From {
		t.Errorf("from mismatch")
	}
	if parsed.WTAddr != original.WTAddr {
		t.Errorf("wtAddr mismatch: %s != %s", parsed.WTAddr, original.WTAddr)
	}
	if parsed.WTCertHash != original.WTCertHash {
		t.Errorf("certHash mismatch")
	}
}

func TestVPNSignalingFlow(t *testing.T) {
	vs := NewVPNSignaling()

	// Track events received
	var receivedEvents []VPNEvent
	vs.OnVPNEvent("vpn-invite", func(e VPNEvent) {
		receivedEvents = append(receivedEvents, e)
	})
	vs.OnVPNEvent("vpn-accept", func(e VPNEvent) {
		receivedEvents = append(receivedEvents, e)
	})

	// Alice creates invite
	invite := vs.CreateVPNInvite("alice_npub", "bob_npub", "192.168.1.1:443", "cert_hash_abc")
	if invite.Type != "vpn-invite" {
		t.Fatalf("expected vpn-invite, got %s", invite.Type)
	}

	// Simulate event arriving via Nostr relay
	vs.HandleIncomingEvent("evt-1", invite)

	// Bob accepts
	accept := vs.CreateVPNAccept("bob_npub", "alice_npub", "192.168.1.1:443", "cert_hash_abc")
	vs.HandleIncomingEvent("evt-2", accept)

	// Wait for async handlers
	time.Sleep(100 * time.Millisecond)

	if len(receivedEvents) != 2 {
		t.Fatalf("expected 2 events, got %d", len(receivedEvents))
	}

	// Check pending invites
	pending := vs.GetPendingInvites()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending invite, got %d", len(pending))
	}
	if pending[0].Type != "vpn-invite" {
		t.Errorf("expected pending invite, got %s", pending[0].Type)
	}

	t.Log("✅ VPN signaling flow: invite → accept works")
}

func TestVPNRequestFlow(t *testing.T) {
	vs := NewVPNSignaling()

	var requests []VPNEvent
	vs.OnVPNEvent("vpn-request", func(e VPNEvent) {
		requests = append(requests, e)
	})

	// Bob requests VPN from Alice
	req := vs.CreateVPNRequest("bob_npub", "alice_npub")
	vs.HandleIncomingEvent("evt-req-1", req)

	time.Sleep(100 * time.Millisecond)

	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].From != "bob_npub" {
		t.Errorf("expected from=bob_npub")
	}

	t.Log("✅ VPN request flow works")
}
