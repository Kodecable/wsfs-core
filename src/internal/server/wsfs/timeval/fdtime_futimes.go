//go:build darwin || dragonfly || freebsd || netbsd || openbsd || solaris || zos

package timeval

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

func SetFDMTime(fd int, ts wsfsprotocol.Timespec) error {
	times, err := unixTimevals(ts)
	if err != nil {
		return err
	}
	return unix.Futimes(fd, times)
}
