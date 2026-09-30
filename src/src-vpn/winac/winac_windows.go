// Package winac applies Windows file/dir ACLs (DACL via SDDL). No-ops on
// non-Windows so callers stay cross-platform (TZ-FINAL §2.1–2.3).
package winac

import (
	"fmt"
	"os/user"

	"golang.org/x/sys/windows"
)

// applyDACL sets a protected DACL (replaces inheritance) on path. If the
// primary SDDL is rejected by advapi32 (server-build SDDL quirks), documented
// fallbacks are tried in order; every attempt is reported via probe.
func applyDACL(path, sddl string, fallbacks ...string) error {
	var lastErr error
	for _, s := range append([]string{sddl}, fallbacks...) {
		sd, err := windows.SecurityDescriptorFromString(s)
		if err != nil {
			lastErr = fmt.Errorf("winac: parse sddl %q: %w", s, err)
			continue
		}
		dacl, _, err := sd.DACL()
		if err != nil || dacl == nil {
			lastErr = fmt.Errorf("winac: dacl %q: %w", s, err)
			continue
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
	return lastErr
}

// sddlVariants documents the per-policy SDDL ladder: canonical form first,
// then bit-equivalent spellings for advapi32 builds that reject generic
// rights strings on file objects.
var (
	pdFallbacks  = []string{"D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FRFX;;;BU)", "D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GRGX;;;BU)"}
	admFallbacks = []string{"D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", "D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"}
)

// ApplyProgramDataDACL: SYSTEM+Admins full, builtin Users read/execute only.
// For C:\ProgramData\Proxi (TZ-FINAL 2.1).
func ApplyProgramDataDACL(path string) error {
	return applyDACL(path,
		"D:PAI(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)(A;OICI;GRGE;;;BU)",
		pdFallbacks...)
}

// ApplyAdminOnlyACL: SYSTEM+Admins only — logs and machine state that no
// regular user should read (TZ-FINAL 2.2).
func ApplyAdminOnlyACL(path string) error {
	return applyDACL(path,
		"D:PAI(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)",
		admFallbacks...)
}

// ApplyUserKeyACL: SYSTEM, Administrators and the calling process user only —
// identity private keys (TZ-FINAL 2.3). Non-owner gets nothing.
func ApplyUserKeyACL(path string) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	primary := "D:PAI(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)(A;OICI;GA;;;" + sid + ")"
	fb1 := "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + sid + ")"
	fb2 := "D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + sid + ")"
	return applyDACL(path, primary, fb1, fb2)
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
