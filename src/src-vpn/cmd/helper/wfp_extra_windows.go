//go:build windows

package main

import (
	"fmt"
	"net/netip"
	"os"

	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

func ruleGUID(data1 uint32) wf.RuleID {
	return wf.RuleID(windows.GUID{Data1: data1, Data4: proxiSublayerGUID.Data4})
}

func addEngageExtras(session *wf.Session) error {
	if exe, err := os.Executable(); err == nil {
		if err = addAppPermit(session, 0x5101, exe, "proxi-permit-helper-v4", wf.LayerALEAuthConnectV4); err != nil {
			return err
		}
		if err = addAppPermit(session, 0x5102, exe, "proxi-permit-helper-v6", wf.LayerALEAuthConnectV6); err != nil {
			return err
		}
		emit("appid-helper")
	}
	if core := installedCorePath(); core != "" {
		if _, err := os.Stat(core); err == nil {
			if err = addAppPermit(session, 0x5103, core, "proxi-permit-core-v4", wf.LayerALEAuthConnectV4); err != nil {
				return err
			}
			if err = addAppPermit(session, 0x5104, core, "proxi-permit-core-v6", wf.LayerALEAuthConnectV6); err != nil {
				return err
			}
			emit("appid-core")
		}
	}
	if err := addNDP(session); err != nil {
		return err
	}
	return addDNSBlocks(session)
}

func addAppPermit(session *wf.Session, id uint32, exe, name string, layer wf.LayerID) error {
	app, err := wf.AppID(exe)
	if err != nil {
		return fmt.Errorf("access_denied: appid: %w", err)
	}
	err = session.AddRule(&wf.Rule{
		ID:       ruleGUID(id),
		Name:     name,
		Layer:    layer,
		Sublayer: wf.SublayerID(proxiSublayerGUID),
		Weight:   14,
		Conditions: []*wf.Match{
			{Field: wf.FieldALEAppID, Op: wf.MatchTypeEqual, Value: app},
			{Field: wf.FieldIPRemotePort, Op: wf.MatchTypeNotEqual, Value: uint16(53)},
		},
		Action: wf.ActionPermit,
	})
	if err == nil {
		return nil
	}
	return session.AddRule(&wf.Rule{
		ID:       ruleGUID(id),
		Name:     name,
		Layer:    layer,
		Sublayer: wf.SublayerID(proxiSublayerGUID),
		Weight:   14,
		Conditions: []*wf.Match{{
			Field: wf.FieldALEAppID,
			Op:    wf.MatchTypeEqual,
			Value: app,
		}},
		Action: wf.ActionPermit,
	})
}

func addNDP(session *wf.Session) error {
	id := uint32(0x6101)
	fe80, err := netip.ParsePrefix("fe80::/10")
	if err != nil {
		return err
	}
	for _, typ := range []uint16{133, 134, 135, 136, 137} {
		for _, layer := range []wf.LayerID{wf.LayerInboundIPPacketV6, wf.LayerOutboundIPPacketV6} {
			rule := &wf.Rule{
				ID:       ruleGUID(id),
				Name:     fmt.Sprintf("proxi-ndp-%d-%d", typ, id),
				Layer:    layer,
				Sublayer: wf.SublayerID(proxiSublayerGUID),
				Weight:   12,
				Conditions: []*wf.Match{
					{Field: wf.FieldIPProtocol, Op: wf.MatchTypeEqual, Value: wf.IPProtoICMPV6},
					{Field: wf.FieldOriginalICMPType, Op: wf.MatchTypeEqual, Value: typ},
					{Field: wf.FieldIPLocalAddress, Op: wf.MatchTypePrefix, Value: fe80},
				},
				Action: wf.ActionPermit,
			}
			if err = session.AddRule(rule); err != nil {
				rule.Conditions = rule.Conditions[:2]
				if err = session.AddRule(rule); err != nil {
					return fmt.Errorf("access_denied: ndp %d: %w", typ, err)
				}
				emit("ndp-permit degraded")
			}
			id++
		}
	}
	emit("ndp-permit 133-137 fe80::/10")
	return nil
}

func addDNSBlocks(session *wf.Session) error {
	id := uint32(0x7101)
	for _, layer := range []wf.LayerID{wf.LayerALEAuthConnectV4, wf.LayerALEAuthConnectV6} {
		for _, proto := range []wf.IPProto{wf.IPProtoUDP, wf.IPProtoTCP} {
			err := session.AddRule(&wf.Rule{
				ID:       ruleGUID(id),
				Name:     fmt.Sprintf("proxi-block-dns-%d", id),
				Layer:    layer,
				Sublayer: wf.SublayerID(proxiSublayerGUID),
				Weight:   9,
				Conditions: []*wf.Match{
					{Field: wf.FieldIPProtocol, Op: wf.MatchTypeEqual, Value: proto},
					{Field: wf.FieldIPRemotePort, Op: wf.MatchTypeEqual, Value: uint16(53)},
				},
				Action: wf.ActionBlock,
			})
			if err != nil {
				return fmt.Errorf("access_denied: dns-block: %w", err)
			}
			id++
		}
	}
	emit("dns-block-53")
	return nil
}
