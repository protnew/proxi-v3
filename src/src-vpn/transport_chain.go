package vpn

import (
	"context"
	"errors"
	"net"
)

// Tunnel is D1: one adapter in DC → WT → UserspaceVPN → Tor → Nostr-degraded.
type Tunnel interface {
	Name() string
	Dial(ctx context.Context, target string) (net.Conn, error)
	UDPMode() string
}

var ErrLegDown = errors.New("transport leg down")

type Chain struct {
	Legs []Tunnel
}

func (c *Chain) Dial(ctx context.Context, target string) (net.Conn, string, error) {
	var last error = ErrLegDown
	for _, leg := range c.Legs {
		conn, err := leg.Dial(ctx, target)
		if err == nil {
			return conn, leg.Name(), nil
		}
		last = err
	}
	return nil, "", last
}

type staticLeg struct {
	name string
	mode string
	dial func(ctx context.Context, target string) (net.Conn, error)
}

func (s staticLeg) Name() string    { return s.name }
func (s staticLeg) UDPMode() string { return s.mode }
func (s staticLeg) Dial(ctx context.Context, target string) (net.Conn, error) {
	if s.dial == nil {
		return nil, ErrLegDown
	}
	return s.dial(ctx, target)
}

// DesktopChain is the confirmed order. Legs without a dialer fail closed.
func DesktopChain(dialers map[string]func(context.Context, string) (net.Conn, error)) *Chain {
	order := []struct{ name, mode string }{
		{"dc", "datagram"},
		{"wt", "stream"},
		{"userspace", "datagram"},
		{"tor", "dropped"},
		{"nostr", "dropped"},
	}
	c := &Chain{}
	for _, o := range order {
		c.Legs = append(c.Legs, staticLeg{name: o.name, mode: o.mode, dial: dialers[o.name]})
	}
	return c
}

// LoopbackEgressIsNotProof marks a same-machine SOCKS check. It is not VPN-108.
func LoopbackEgressIsNotProof(ip string) bool {
	return ip == "127.0.0.1" || ip == "::1" || ip == ""
}
