//go:build windows

package wsfs

import (
	"errors"
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/windows"
)

func osErrCode_osOverride(err error) (uint8, bool) {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return 0, false
	}

	switch errno {
	case windows.ERROR_SHARING_VIOLATION, windows.ERROR_LOCK_VIOLATION:
		return wsfsprotocol.ErrorBusy, true
	default:
		return 0, false
	}
}
