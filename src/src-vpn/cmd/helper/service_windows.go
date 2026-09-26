//go:build windows

package main

import (
	"errors"
	"strings"
	"time"

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
	s, err := m.OpenService(helperServiceName)
	if err != nil {
		s, err = m.CreateService(helperServiceName, exe, mgr.Config{
			DisplayName:      "Proxi Helper",
			Description:      "Proxi elevated helper (wintun + WFP)",
			StartType:        mgr.StartAutomatic,
			ServiceStartName: "LocalSystem",
		}, "--service")
		if err != nil {
			return err
		}
	}
	defer s.Close()
	if err := s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
	}, 24*60*60); err != nil {
		return err
	}
	if err := rememberCorePath(exe); err != nil {
		emit("core-path-registry " + err.Error())
	}
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
			removeInstallArtifacts(installDir())
			return forgetCorePath()
		}
		return err
	}
	defer s.Close()
	st, qerr := s.Query()
	if qerr == nil && st.State == svc.Running {
		_, _ = s.Control(svc.Stop)
	}
	err = s.Delete()
	removeInstallArtifacts(installDir())
	if ferr := forgetCorePath(); ferr != nil {
		return ferr
	}
	return err
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
