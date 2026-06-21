//go:build (freebsd && (386 || mips || mipsle)) || (linux && (386 || arm || mips || mipsle || ppc))

package timeval

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

func unixTimespecs(ts wsfsprotocol.Timespec) ([]unix.Timespec, error) {
	if !ValidNsec(ts.Nanoseconds) {
		return nil, unix.EINVAL
	}
	if !FitsInt32(ts.Seconds) {
		return nil, unix.ERANGE
	}
	t := unix.Timespec{Sec: int32(ts.Seconds), Nsec: int32(ts.Nanoseconds)}
	return []unix.Timespec{t, t}, nil
}

func unixTimevals(ts wsfsprotocol.Timespec) ([]unix.Timeval, error) {
	sec, usec, err := TimevalParts(ts)
	if err != nil {
		return nil, err
	}
	if !FitsInt32(sec) {
		return nil, unix.ERANGE
	}
	t := unix.Timeval{Sec: int32(sec), Usec: int32(usec)}
	return []unix.Timeval{t, t}, nil
}
