package vpn

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

// wtAllowPrivate is test-only. Metadata stays denied.
var wtAllowPrivate atomic.Bool

func setWTAllowPrivateForTest(v bool) { wtAllowPrivate.Store(v) }

const wtMaxStreamsPerSession = 8

var metadataIP = net.ParseIP("169.254.169.254")

// ipDenied reports whether a literal IP hits the deny-list.
func ipDenied(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.Equal(metadataIP) {
		return true
	}
	if wtAllowPrivate.Load() {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// aclResolveTarget checks the CONNECT target. Hostnames are resolved and EVERY
// A/AAAA is checked (DNS-rebinding guard): any denied result denies the whole
// target. Returns a pinned IP for the caller's dial so the checked address is
// the one actually connected (no TOCTOU re-resolve).
func aclResolveTarget(target string) (pinned net.IP, denied bool) {
	host := target
	port := ""
	if h, p, err := net.SplitHostPort(target); err == nil {
		host, port = h, p
	}
	host = strings.Trim(host, "[]")
	low := strings.ToLower(host)
	if low == "localhost" || low == "metadata.google.internal" || strings.HasSuffix(low, ".localhost") {
		return nil, true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip, ipDenied(ip)
	}
	// Non-IP hostname: resolve, then check every answer.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, true
	}
	for _, ip := range ips {
		if ipDenied(ip) {
			return nil, true
		}
	}
	_ = port
	return ips[0], false
}

// targetDenied blocks SSRF: loopback, LAN, link-local, metadata, and any
// hostname resolving into a denied range.
func targetDenied(target string) bool {
	_, denied := aclResolveTarget(target)
	return denied
}

// pinnedTarget rewrites host:port to pinnedIP:port for a TOCTOU-safe dial.
func pinnedTarget(target string, ip net.IP) string {
	_, port, err := net.SplitHostPort(target)
	if err != nil {
		return target
	}
	return net.JoinHostPort(ip.String(), port)
}

type streamBudget struct {
	n atomic.Int32
}

func (b *streamBudget) take() bool {
	if b.n.Add(1) > wtMaxStreamsPerSession {
		b.n.Add(-1)
		return false
	}
	return true
}
