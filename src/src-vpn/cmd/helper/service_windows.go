//go:build windows

package main

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

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

func uninstallHelperService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(helperServiceName)
	if err != nil {
		if serviceGone(err) {
			return nil
		}
		return err
	}
	defer s.Close()
	st, qerr := s.Query()
	if qerr == nil && st.State == svc.Running {
		_, _ = s.Control(svc.Stop)
	}
	return s.Delete()
}

func serviceGone(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "1060") || strings.Contains(msg, "does not exist")
}
