//go:build unix && !(linux && (386 || arm || mips || mipsle || ppc))

package timeval

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

func SetPathMTime(path string, ts wsfsprotocol.Timespec) error {
	times, err := unixTimespecs(ts)
	if err != nil {
		return err
	}
	return unix.UtimesNanoAt(unix.AT_FDCWD, path, times, 0)
}
