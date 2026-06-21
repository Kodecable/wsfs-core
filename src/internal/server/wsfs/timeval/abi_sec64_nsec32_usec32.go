//go:build (freebsd && arm) || (netbsd && (386 || arm)) || (openbsd && (386 || arm))

package timeval

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

func unixTimespecs(ts wsfsprotocol.Timespec) ([]unix.Timespec, error) {
	if !ValidNsec(ts.Nanoseconds) {
		return nil, unix.EINVAL
	}
	t := unix.Timespec{Sec: ts.Seconds, Nsec: int32(ts.Nanoseconds)}
	return []unix.Timespec{t, t}, nil
}

func unixTimevals(ts wsfsprotocol.Timespec) ([]unix.Timeval, error) {
	sec, usec, err := TimevalParts(ts)
	if err != nil {
		return nil, err
	}
	t := unix.Timeval{Sec: sec, Usec: int32(usec)}
	return []unix.Timeval{t, t}, nil
}
