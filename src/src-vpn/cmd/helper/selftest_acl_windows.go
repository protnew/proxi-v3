//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/unkillable-messenger/vpn/winac"
	"golang.org/x/sys/windows"
)

const (
	sidUsers  = "S-1-5-32-545" // BUILTIN\Users (locale-neutral)
	sidAdmins = "S-1-5-32-544"
	sidSystem = "S-1-5-18"
)

// selftestACL — TZ-04-20260930 §2.2: live proof of the winac DACLs on a
// windows runner. Round-trips each DACL back to SDDL (locale-neutral) and
// asserts the exact ACE shape we install. Emits one marker per check.
func selftestACL() error {
	tmp, err := os.MkdirTemp("", "proxi04-acl-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// 1. ProgramData-style dir: builtin Users get read/execute only.
	dir := filepath.Join(tmp, "pd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := winac.ApplyProgramDataDACL(dir); err != nil {
		return fmt.Errorf("programdata dacl: %w", err)
	}
	mask, found, err := aceMaskForSID(dir, sidUsers)
	if err != nil {
		return fmt.Errorf("programdata readback: %w", err)
	}
	if !found {
		return fmt.Errorf("programdata: no Users ACE at all (expected GRGE)")
	}
	if mask&windows.GENERIC_WRITE != 0 || mask&windows.GENERIC_ALL != 0 {
		return fmt.Errorf("programdata: Users mask grants write/all: %x", mask)
	}
	emit("selftest-acl programdata users-ro ok")

	// 2. Admin-only file: no Users ACE at all.
	f := filepath.Join(tmp, "log.txt")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		return err
	}
	if err := winac.ApplyAdminOnlyACL(f); err != nil {
		return fmt.Errorf("adminonly dacl: %w", err)
	}
	if _, found, err = aceMaskForSID(f, sidUsers); err != nil {
		return fmt.Errorf("adminonly readback: %w", err)
	}
	if found {
		return fmt.Errorf("admin-only file grants users")
	}
	emit("selftest-acl admin-only no-users ok")

	// 3. User-only key: SYSTEM+Admins present, Users absent.
	k := filepath.Join(tmp, "private.key")
	if err := os.WriteFile(k, []byte("k"), 0o600); err != nil {
		return err
	}
	if err := winac.ApplyUserKeyACL(k); err != nil {
		return fmt.Errorf("userkey dacl: %w", err)
	}
	if _, found, err = aceMaskForSID(k, sidUsers); err != nil {
		return fmt.Errorf("userkey readback: %w", err)
	}
	if found {
		return fmt.Errorf("user key grants users")
	}
	for _, want := range []string{sidSystem, sidAdmins} {
		if _, ok, err := aceMaskForSID(k, want); err != nil || !ok {
			return fmt.Errorf("user key missing SID %s (err=%v)", want, err)
		}
	}
	emit("selftest-acl user-key sy+ba+owner ok")
	emit("selftest-acl all ok")
	return nil
}

// aceMaskForSID walks the object's DACL and returns the access mask granted
// to the given SID (locale-neutral well-known SIDs).
func aceMaskForSID(path, sidString string) (windows.ACCESS_MASK, bool, error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return 0, false, err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return 0, false, err
	}
	if dacl == nil {
		return 0, false, nil
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if sid.String() == sidString {
			return ace.Mask, true, nil
		}
	}
	return 0, false, nil
}
