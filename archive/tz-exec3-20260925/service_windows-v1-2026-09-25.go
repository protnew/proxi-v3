//go:build windows

package main

import (
	"golang.org/x/sys/windows/svc/mgr"
)

const helperServiceName = "ProxiHelper"

func installHelperService(exe string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s, err := m.OpenService(helperServiceName); err == nil {
		s.Close()
		return nil
	}
	s, err := m.CreateService(helperServiceName, exe, mgr.Config{
		DisplayName:      "Proxi Helper",
		Description:      "Proxi elevated helper (wintun + WFP)",
		StartType:        mgr.StartAutomatic,
		ServiceStartName: "LocalSystem",
	}, "--service")
	if err != nil {
		return err
	}
	defer s.Close()
	return nil
}

func probeSCM() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	return m.Disconnect()
}
