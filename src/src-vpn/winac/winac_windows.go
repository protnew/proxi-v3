// Package winac applies Windows file/dir ACLs (DACL via SDDL). No-ops on
// non-Windows so callers stay cross-platform (TZ-FINAL §2.1–2.3).
package winac

import (
	"fmt"
	"os/user"

	"golang.org/x/sys/windows"
)

// applyDACL sets a protected DACL (replaces inheritance) on path.
func applyDACL(path, sddl string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("winac: parse sddl: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("winac: dacl: %w", err)
	}
	if dacl == nil {
		return fmt.Errorf("winac: nil dacl for %s", sddl)
	}
	err = windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil,
	)
	if err != nil {
		return fmt.Errorf("winac: set on %s: %w", path, err)
	}
	return nil
}

// ApplyProgramDataDACL: SYSTEM+Admins full, builtin Users read/execute only.
// For C:\ProgramData\Proxi (TZ-FINAL 2.1).
func ApplyProgramDataDACL(path string) error {
	return applyDACL(path, "D:PAI(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)(A;OICI;GRGE;;;BU)")
}

// ApplyAdminOnlyACL: SYSTEM+Admins only — logs and machine state that no
// regular user should read (TZ-FINAL 2.2).
func ApplyAdminOnlyACL(path string) error {
	return applyDACL(path, "D:PAI(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)")
}

// ApplyUserKeyACL: SYSTEM, Administrators and the calling process user only —
// identity private keys (TZ-FINAL 2.3). Non-owner gets nothing.
func ApplyUserKeyACL(path string) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	return applyDACL(path, "D:PAI(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)(A;OICI;GA;;;"+sid+")")
}

func currentUserSID() (string, error) {
	// os/user on Windows returns the SID string in Uid.
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("winac: current user: %w", err)
	}
	if u.Uid == "" {
		return "", fmt.Errorf("winac: current user has empty SID")
	}
	return u.Uid, nil
}
