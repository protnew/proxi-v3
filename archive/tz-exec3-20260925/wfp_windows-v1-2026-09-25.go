//go:build windows

package main

import (
	"fmt"
	"strings"

	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

var proxiProviderID = wf.ProviderID(windows.GUID{
	Data1: 0x50524f58, Data2: 0x2026, Data3: 0x0925,
	Data4: [8]byte{0x50, 0x52, 0x4f, 0x58, 0x49, 0x4b, 0x53, 0x01},
})

var proxiSublayerGUID = windows.GUID{
	Data1: 0x50524f58, Data2: 0x4b53, Data3: 0x2026,
	Data4: [8]byte{0x50, 0x52, 0x4f, 0x58, 0x49, 0x4b, 0x53, 0x02},
}

func killSwitchSublayer() *wf.Sublayer {
	return &wf.Sublayer{
		ID:          wf.SublayerID(proxiSublayerGUID),
		Name:        WFPSublayerName,
		Description: "Proxi persistent kill-switch",
		Persistent:  true,
		Provider:    proxiProviderID,
		Weight:      0xFFFF,
	}
}

func killSwitchRules(tunLUID uint64) []*wf.Rule {
	sub := wf.SublayerID(proxiSublayerGUID)
	layers := []wf.LayerID{
		wf.LayerALEAuthConnectV4, wf.LayerALEAuthConnectV6,
		wf.LayerALEAuthRecvAcceptV4, wf.LayerALEAuthRecvAcceptV6,
	}
	var rules []*wf.Rule
	for i, layer := range layers {
		rules = append(rules, &wf.Rule{
			ID:         wf.RuleID(windows.GUID{Data1: 0x1000 + uint32(i), Data4: proxiSublayerGUID.Data4}),
			Name:       "PROXI_KILLSWITCH permit loopback",
			Layer:      layer,
			Sublayer:   sub,
			Weight:     15,
			Action:     wf.ActionPermit,
			Persistent: true,
			Conditions: []*wf.Match{{
				Field: wf.FieldFlags,
				Op:    wf.MatchTypeFlagsAllSet,
				Value: wf.ConditionFlagIsLoopback,
			}},
		})
		rules = append(rules, &wf.Rule{
			ID:         wf.RuleID(windows.GUID{Data1: 0x2000 + uint32(i), Data4: proxiSublayerGUID.Data4}),
			Name:       "PROXI_KILLSWITCH permit dhcp",
			Layer:      layer,
			Sublayer:   sub,
			Weight:     12,
			Action:     wf.ActionPermit,
			Persistent: true,
			Conditions: []*wf.Match{{
				Field: wf.FieldIPRemotePort,
				Op:    wf.MatchTypeEqual,
				Value: uint16(67),
			}},
		})
		if tunLUID != 0 {
			rules = append(rules, &wf.Rule{
				ID:         wf.RuleID(windows.GUID{Data1: 0x3000 + uint32(i), Data4: proxiSublayerGUID.Data4}),
				Name:       "PROXI_KILLSWITCH permit tun",
				Layer:      layer,
				Sublayer:   sub,
				Weight:     15,
				Action:     wf.ActionPermit,
				Persistent: true,
				Conditions: []*wf.Match{{
					Field: wf.FieldIPLocalInterface,
					Op:    wf.MatchTypeEqual,
					Value: tunLUID,
				}},
			})
		}
		rules = append(rules, &wf.Rule{
			ID:         wf.RuleID(windows.GUID{Data1: 0x4000 + uint32(i), Data4: proxiSublayerGUID.Data4}),
			Name:       "PROXI_KILLSWITCH block",
			Layer:      layer,
			Sublayer:   sub,
			Weight:     0,
			Action:     wf.ActionBlock,
			HardAction: true,
			Persistent: true,
		})
	}
	return rules
}

func installKillSwitch(tunLUID uint64) error {
	session, err := wf.New(&wf.Options{
		Name:        "PROXI",
		Description: "Proxi kill-switch",
		Dynamic:     false,
	})
	if err != nil {
		return err
	}
	defer session.Close()
	_ = session.AddProvider(&wf.Provider{
		ID:          proxiProviderID,
		Name:        "PROXI",
		Description: "Proxi helper",
		Persistent:  true,
	})
	if err := session.AddSublayer(killSwitchSublayer()); err != nil && !alreadyExists(err) {
		return fmt.Errorf("sublayer: %w", err)
	}
	for _, rule := range killSwitchRules(tunLUID) {
		if err := session.AddRule(rule); err != nil && !alreadyExists(err) {
			return fmt.Errorf("rule %s: %w", rule.Name, err)
		}
	}
	return nil
}

func alreadyExists(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "already exists") || strings.Contains(s, "ERROR_ALREADY_EXISTS") || strings.Contains(s, "0x80320009")
}
