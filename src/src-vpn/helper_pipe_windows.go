//go:build windows

package vpn

import (
	"io"

	"github.com/Microsoft/go-winio"
)

func init() {
	dialHelperPipe = func() (io.ReadWriteCloser, error) {
		return winio.DialPipe(`\\.\pipe\Proxi04VpnHelper`, nil)
	}
}
