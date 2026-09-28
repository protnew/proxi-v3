package vpn

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTURN_RequestFormat(t *testing.T) {
	// No transport hook: the request should be built and recorded but not sent.
	SetTURNTransport(nil)
	defer SetTURNTransport(nil)

	alloc, err := RequestTURNAllocation("turn.example.com:3478", "alice", "s3cr3t")
	if err != nil {
		t.Fatalf("RequestTURNAllocation: %v", err)
	}

	req := alloc.RawRequest()
	if req == "" {
		t.Fatal("expected non-empty raw request")
	}

	// The request must contain the key ALLOCATE fields.
	mustContain := []string{
		"ALLOCATE",
		"turn.example.com:3478",
		"txn=",
		"lifetime=",
		"username=alice",
		"realm=turn.example.com",
		"integrity=",
	}
	for _, sub := range mustContain {
		if !strings.Contains(req, sub) {
			t.Errorf("request missing %q\nrequest: %s", sub, req)
		}
	}
	// Default lifetime should be present in seconds.
	if !strings.Contains(req, fmt.Sprintf("lifetime=%ds", DefaultTURNLifetime)) {
		t.Errorf("request should request default lifetime %ds", DefaultTURNLifetime)
	}

	// Without a transport the allocation stays in the requesting state.
	if alloc.State() != TURNRequesting {
		t.Errorf("state = %s, want requesting", alloc.State())
	}
	if alloc.Lifetime() != time.Duration(DefaultTURNLifetime)*time.Second {
		t.Errorf("lifetime = %v, want %v", alloc.Lifetime(), time.Duration(DefaultTURNLifetime)*time.Second)
	}
}

func TestTURN_ValidationErrors(t *testing.T) {
	SetTURNTransport(nil)
	defer SetTURNTransport(nil)

	if _, err := RequestTURNAllocation("", "u", "p"); err == nil {
		t.Error("expected error for empty server")
	}
	if _, err := RequestTURNAllocation("turn.example.com:3478", "", "p"); err == nil {
		t.Error("expected error for empty username")
	}
	if _, err := RequestTURNAllocation("turn.example.com:3478", "u", ""); err == nil {
		t.Error("expected error for empty password")
	}
}

func TestTURN_AllocatedViaTransport(t *testing.T) {
	// Fake transport: echo back a relayed address in the response.
	SetTURNTransport(func(server, request string) (string, error) {
		if !strings.HasPrefix(request, "ALLOCATE") {
			return "", fmt.Errorf("unexpected request %q", request)
		}
		return "ALLOCATE-OK relayed=203.0.113.10:49152 lifetime=600", nil
	})
	defer SetTURNTransport(nil)

	alloc, err := RequestTURNAllocation("turn.example.com:3478", "alice", "s3cr3t")
	if err != nil {
		t.Fatalf("RequestTURNAllocation: %v", err)
	}
	if alloc.State() != TURNAllocated {
		t.Fatalf("state = %s, want allocated", alloc.State())
	}
	if alloc.RelayedAddr() != "203.0.113.10:49152" {
		t.Errorf("relayed = %q, want 203.0.113.10:49152", alloc.RelayedAddr())
	}
	if !alloc.ExpiresAt().After(time.Now()) {
		t.Error("allocation should expire in the future")
	}

	// Release should revoke the allocation.
	alloc.Release()
	if alloc.State() != TURNRevoked {
		t.Errorf("after release state = %s, want revoked", alloc.State())
	}
	if alloc.RelayedAddr() != "" {
		t.Errorf("relayed addr should be cleared after release, got %q", alloc.RelayedAddr())
	}
}

func TestTURN_TransportFailure(t *testing.T) {
	SetTURNTransport(func(server, request string) (string, error) {
		return "", fmt.Errorf("connection refused")
	})
	defer SetTURNTransport(nil)

	alloc, err := RequestTURNAllocation("turn.example.com:3478", "alice", "s3cr3t")
	if err == nil {
		t.Fatal("expected error when transport fails")
	}
	if alloc.State() != TURNFailed {
		t.Errorf("state = %s, want failed", alloc.State())
	}
}

func TestTURN_StateString(t *testing.T) {
	expectations := map[TURNState]string{
		TURNIdle:       "idle",
		TURNRequesting: "requesting",
		TURNAllocated:  "allocated",
		TURNFailed:     "failed",
		TURNRevoked:    "revoked",
	}
	for s, want := range expectations {
		if got := s.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", s, got, want)
		}
	}
}
