//go:build ignore

package vpn

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// T42BConfig is the one-button Android system-VPN control plane (T42-B).
// DefaultRoute is always 0.0.0.0/0 — partial tunnel is not this product.
type T42BConfig struct {
	Listen       string // empty → 127.0.0.1:0 (ephemeral, for tests)
	ExitSOCKS    string // empty → auto local exit (no human peer)
	DefaultRoute string
	Session      string
	KillSwitch   bool
}

func DefaultT42BConfig() T42BConfig {
	return T42BConfig{
		Listen:       "127.0.0.1:0",
		DefaultRoute: "0.0.0.0/0",
		Session:      "IndestructibleVPN",
		KillSwitch:   true,
	}
}

// T42BStatus is what the UI button reads.
type T42BStatus struct {
	State     string
	SocksAddr string
	AutoExit  bool
	ChannelUp bool
	KillOn    bool
}

// T42BButton starts a local SOCKS5 exit automatically so tests (and the
// developer) do not have to organise a peer channel by hand.
// Product later: ExitSOCKS = SOCKS on the DataChannel side of a real peer.
type T42BButton struct {
	cfg       T42BConfig
	mu        sync.Mutex
	socks     *SOCKS5Server
	autoExit  bool
	channelUp atomic.Bool
	started   atomic.Bool
}

func NewT42BButton(cfg T42BConfig) *T42BButton {
	if cfg.DefaultRoute == "" {
		cfg.DefaultRoute = "0.0.0.0/0"
	}
	if cfg.Session == "" {
		cfg.Session = "IndestructibleVPN"
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:0"
	}
	return &T42BButton{cfg: cfg}
}

func (b *T42BButton) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started.Load() && b.socks != nil && b.socks.IsRunning() {
		return nil
	}

	listen := b.cfg.Listen
	if listen == "127.0.0.1:0" || listen == ":0" {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		listen = ln.Addr().String()
		_ = ln.Close()
	}

	upstream := b.cfg.ExitSOCKS
	auto := upstream == ""
	srv := NewSOCKS5Server(listen, upstream)
	if err := srv.Start(); err != nil {
		return fmt.Errorf("t42b socks start: %w", err)
	}
	if b.socks != nil {
		_ = b.socks.Stop()
	}
	b.socks = srv
	b.autoExit = auto
	b.channelUp.Store(true)
	b.started.Store(true)
	return nil
}

func (b *T42BButton) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started.Store(false)
	b.channelUp.Store(false)
	if b.socks != nil {
		_ = b.socks.Stop()
		b.socks = nil
	}
	return nil
}

func (b *T42BButton) SetChannelUp(up bool) {
	b.channelUp.Store(up)
}

func (b *T42BButton) Status() T42BStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := T42BStatus{
		State:     "disconnected",
		AutoExit:  b.autoExit,
		ChannelUp: b.channelUp.Load(),
		KillOn:    b.cfg.KillSwitch,
	}
	if b.started.Load() && b.socks != nil && b.socks.IsRunning() {
		st.State = "connected"
		st.SocksAddr = b.socks.Addr()
	}
	return st
}

// Dial opens target through the button's SOCKS5. If the DataChannel is down
// the kill switch refuses the dial — no leak to the operator network.
func (b *T42BButton) Dial(target string) (net.Conn, error) {
	if !b.started.Load() {
		return nil, fmt.Errorf("t42b: not started")
	}
	if !b.channelUp.Load() {
		return nil, fmt.Errorf("t42b: kill switch — channel down")
	}
	b.mu.Lock()
	srv := b.socks
	b.mu.Unlock()
	if srv == nil || !srv.IsRunning() {
		return nil, fmt.Errorf("t42b: kill switch — socks down")
	}
	return dialViaSOCKS5(srv.Addr(), target, 8*time.Second)
}
