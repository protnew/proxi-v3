//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	if err := addEngageExtras(session); err != nil {
		return err
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

func knownKillSwitchRuleIDs() []wf.RuleID {
	ids := make([]wf.RuleID, 0, 16)
	for i := uint32(0); i < 4; i++ {
		for _, base := range []uint32{0x1000, 0x2000, 0x3000, 0x4000} {
			ids = append(ids, wf.RuleID(windows.GUID{Data1: base + i, Data4: proxiSublayerGUID.Data4}))
		}
	}
	for _, base := range []uint32{0x5100, 0x6100, 0x7100} {
		for i := uint32(0); i < 16; i++ {
			ids = append(ids, wf.RuleID(windows.GUID{Data1: base + i, Data4: proxiSublayerGUID.Data4}))
		}
	}
	return ids
}

func missingWFP(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "not found") || strings.Contains(s, "does not exist") || strings.Contains(s, "0x80320003") || strings.Contains(s, "0x80320007") || strings.Contains(s, "0x80320008")
}

func enumKillSwitch() (string, int, error) {
	session, err := wf.New(&wf.Options{Name: "PROXI", Description: "Proxi kill-switch enum", Dynamic: false})
	if err != nil {
		return "", 0, err
	}
	defer session.Close()
	want := wf.SublayerID(proxiSublayerGUID)
	subs, err := session.Sublayers()
	if err != nil {
		return "", 0, err
	}
	found := false
	guid := windows.GUID(proxiSublayerGUID).String()
	for _, sl := range subs {
		if sl == nil {
			continue
		}
		if sl.ID == want || sl.Name == WFPSublayerName {
			found = true
			guid = windows.GUID(sl.ID).String()
		}
	}
	if !found {
		return guid, 0, fmt.Errorf("sublayer %s not in WFP enum", WFPSublayerName)
	}
	rules, err := session.Rules()
	if err != nil {
		return guid, 0, err
	}
	n := 0
	for _, r := range rules {
		if r != nil && r.Sublayer == want {
			n++
		}
	}
	return guid, n, nil
}

func removeKillSwitch() error {
	session, err := wf.New(&wf.Options{Name: "PROXI", Description: "Proxi kill-switch remove", Dynamic: false})
	if err != nil {
		return err
	}
	defer session.Close()
	var problems []string
	for _, id := range knownKillSwitchRuleIDs() {
		if err := session.DeleteRule(id); err != nil && !missingWFP(err) {
			problems = append(problems, err.Error())
		}
	}
	if err := session.DeleteSublayer(wf.SublayerID(proxiSublayerGUID)); err != nil && !missingWFP(err) {
		problems = append(problems, "sublayer: "+err.Error())
	}
	if err := session.DeleteProvider(proxiProviderID); err != nil && !missingWFP(err) {
		problems = append(problems, "provider: "+err.Error())
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

func rollbackNotePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "WFP-ROLLBACK.txt")
}

func armRollbackNote() {
	path := rollbackNotePath()
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte("helper.exe --killswitch-remove\nnetsh wfp show filters\n"), 0600)
}

func clearRollbackNote() {
	path := rollbackNotePath()
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

func printRollback() {
	emit("ROLLBACK: helper.exe --killswitch-remove")
	emit("ROLLBACK: netsh wfp show filters")
}

func runKillSwitchTest() error {
	armRollbackNote()
	if err := installKillSwitch(0); err != nil {
		return err
	}
	guid, n, err := enumKillSwitch()
	if err != nil {
		_ = removeKillSwitch()
		printRollback()
		return err
	}
	emit(fmt.Sprintf("sublayer GUID=%s filters=%d", guid, n))
	if n == 0 {
		_ = removeKillSwitch()
		printRollback()
		return fmt.Errorf("enum saw sublayer but 0 filters")
	}
	if err := removeKillSwitch(); err != nil {
		printRollback()
		return err
	}
	if _, n2, err2 := enumKillSwitch(); err2 == nil {
		printRollback()
		return fmt.Errorf("sublayer still present filters=%d", n2)
	}
	clearRollbackNote()
	emit("killswitch-removed")
	return nil
}
