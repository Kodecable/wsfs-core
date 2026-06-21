//go:build linux && !(386 || arm || mips || mipsle || ppc)

package timeval

import (
	"path/filepath"
	"strconv"

	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

func SetFDMTime(fd int, ts wsfsprotocol.Timespec) error {
	times, err := unixTimespecs(ts)
	if err != nil {
		return err
	}
	return unix.UtimesNanoAt(unix.AT_FDCWD, filepath.Join("/proc/self/fd", strconv.Itoa(fd)), times, 0)
}
