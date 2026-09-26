//go:build windows

package main

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

func forgetCorePath() error {
	err := registry.DeleteKey(registry.LOCAL_MACHINE, `SOFTWARE\Proxi`)
	if err == nil || errors.Is(err, registry.ErrNotExist) {
		emit("corepath-gone")
		return nil
	}
	k, oerr := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Proxi`, registry.SET_VALUE)
	if oerr != nil {
		if errors.Is(oerr, registry.ErrNotExist) {
			emit("corepath-gone")
			return nil
		}
		return oerr
	}
	defer k.Close()
	verr := k.DeleteValue("CorePath")
	if verr != nil && !errors.Is(verr, registry.ErrNotExist) {
		return verr
	}
	emit("corepath-gone")
	return nil
}
