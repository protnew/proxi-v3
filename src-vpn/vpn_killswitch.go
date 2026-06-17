package vpn

import (
	"fmt"
	"sync"
)

// KillSwitchState describes the state of the kill switch.
type KillSwitchState int

const (
	// KillSwitchInactive means traffic is allowed to flow normally.
	KillSwitchInactive KillSwitchState = iota
	// KillSwitchActive means the kill switch firewall rules are installed and
	// all non-tunnel traffic is blocked.
	KillSwitchActive
)

func (s KillSwitchState) String() string {
	switch s {
	case KillSwitchActive:
		return "active"
	case KillSwitchInactive:
		return "inactive"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// DefaultKillSwitchChain is the iptables chain the kill switch operates on.
const DefaultKillSwitchChain = "PROXI_KILLSWITCH"

// KillSwitch manages a set of iptables rules that block all traffic except
// that flowing through the VPN tunnel interface.
//
// For safety and testability, the iptables rules are only *generated and
// stored* — they are never executed. A real deployment would apply them via an
// Executor function.
type KillSwitch struct {
	mu sync.Mutex

	state      KillSwitchState
	tunnelIf   string // protected tunnel interface (e.g. "tun0")
	chain      string // iptables chain name
	allowedIfs []string

	// rules holds the iptables commands that would be installed (Enable) or
	// removed (Disable). The slice is append-only across cycles for inspection.
	rules []string

	// applied is the subset of rules currently considered "installed".
	applied []string

	// Executor, if set, is invoked with each rule. When nil, rules are only
	// stored. This indirection keeps the kill switch safe by default.
	Executor func(rule string) error
}

// NewKillSwitch creates a KillSwitch protecting the given tunnel interface.
// If tunnelIf is empty, "tun0" is assumed.
func NewKillSwitch(tunnelIf string) *KillSwitch {
	if tunnelIf == "" {
		tunnelIf = "tun0"
	}
	return &KillSwitch{
		state:    KillSwitchInactive,
		tunnelIf: tunnelIf,
		chain:    DefaultKillSwitchChain,
	}
}

// State returns the current kill switch state.
func (ks *KillSwitch) State() KillSwitchState {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	return ks.state
}

// IsActive reports whether the kill switch is currently active.
func (ks *KillSwitch) IsActive() bool {
	return ks.State() == KillSwitchActive
}

// AllowInterface marks an additional interface (e.g. "lo" or a LAN iface) whose
// traffic should remain permitted even when the kill switch is active.
func (ks *KillSwitch) AllowInterface(iface string) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.allowedIfs = append(ks.allowedIfs, iface)
}

// buildRules constructs the iptables rule set for the active state.
func (ks *KillSwitch) buildRules() []string {
	rules := []string{
		fmt.Sprintf("iptables -N %s", ks.chain),
		// Allow loopback.
		fmt.Sprintf("iptables -A %s -i lo -j ACCEPT", ks.chain),
		// Allow established/related traffic.
		fmt.Sprintf("iptables -A %s -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT", ks.chain),
		// Allow traffic through the tunnel interface.
		fmt.Sprintf("iptables -A %s -o %s -j ACCEPT", ks.chain, ks.tunnelIf),
		fmt.Sprintf("iptables -A %s -i %s -j ACCEPT", ks.chain, ks.tunnelIf),
	}
	// Explicitly allowed interfaces.
	for _, iface := range ks.allowedIfs {
		rules = append(rules, fmt.Sprintf("iptables -A %s -o %s -j ACCEPT", ks.chain, iface))
		rules = append(rules, fmt.Sprintf("iptables -A %s -i %s -j ACCEPT", ks.chain, iface))
	}
	// Drop everything else.
	rules = append(rules, fmt.Sprintf("iptables -A %s -j DROP", ks.chain))
	// Hook the chain into OUTPUT.
	rules = append(rules, fmt.Sprintf("iptables -A OUTPUT -j %s", ks.chain))
	return rules
}

// teardownRules constructs the rule set that undoes an active kill switch.
func (ks *KillSwitch) teardownRules() []string {
	return []string{
		fmt.Sprintf("iptables -D OUTPUT -j %s", ks.chain),
		fmt.Sprintf("iptables -F %s", ks.chain),
		fmt.Sprintf("iptables -X %s", ks.chain),
	}
}

// run applies each rule via the Executor (if any) and records it.
func (ks *KillSwitch) run(rules []string) error {
	for _, r := range rules {
		if ks.Executor != nil {
			if err := ks.Executor(r); err != nil {
				return fmt.Errorf("apply rule %q: %w", r, err)
			}
		}
		ks.rules = append(ks.rules, r)
	}
	return nil
}

// Enable installs the kill switch firewall rules and marks the switch active.
// Calling Enable when already active is a safe no-op that returns nil.
func (ks *KillSwitch) Enable() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.state == KillSwitchActive {
		// Double-enable is safe: nothing to do.
		return nil
	}

	rules := ks.buildRules()
	if err := ks.run(rules); err != nil {
		return err
	}
	ks.applied = rules
	ks.state = KillSwitchActive
	return nil
}

// Disable removes the kill switch firewall rules and marks the switch inactive.
// Calling Disable when already inactive is a safe no-op that returns nil.
func (ks *KillSwitch) Disable() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.state == KillSwitchInactive {
		return nil
	}

	rules := ks.teardownRules()
	if err := ks.run(rules); err != nil {
		return err
	}
	ks.applied = nil
	ks.state = KillSwitchInactive
	return nil
}

// Rules returns a copy of every rule ever generated (enable + disable across
// cycles), in order. Useful for inspection and testing.
func (ks *KillSwitch) Rules() []string {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	out := make([]string, len(ks.rules))
	copy(out, ks.rules)
	return out
}

// AppliedRules returns a copy of the rules currently considered installed
// (non-empty only while the switch is active).
func (ks *KillSwitch) AppliedRules() []string {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	out := make([]string, len(ks.applied))
	copy(out, ks.applied)
	return out
}

// TunnelInterface returns the protected tunnel interface name.
func (ks *KillSwitch) TunnelInterface() string {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	return ks.tunnelIf
}
