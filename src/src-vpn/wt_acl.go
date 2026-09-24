package vpn

import (
	"net"
	"strings"
	"sync/atomic"
)

// wtAllowPrivate is test-only. Metadata stays denied.
var wtAllowPrivate atomic.Bool

func setWTAllowPrivateForTest(v bool) { wtAllowPrivate.Store(v) }

const wtMaxStreamsPerSession = 8

// targetDenied blocks SSRF: loopback, LAN, link-local, metadata.
func targetDenied(target string) bool {
	host := target
	if h, _, err := net.SplitHostPort(target); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	low := strings.ToLower(host)
	if low == "localhost" || low == "metadata.google.internal" || strings.HasSuffix(low, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	if wtAllowPrivate.Load() {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	return false
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
