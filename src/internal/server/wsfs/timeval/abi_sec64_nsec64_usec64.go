//go:build dragonfly || solaris || zos || (freebsd && (amd64 || arm64 || riscv64)) || (linux && !(386 || arm || mips || mipsle || ppc)) || (openbsd && !(386 || arm))

package timeval

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

func unixTimespecs(ts wsfsprotocol.Timespec) ([]unix.Timespec, error) {
	if !ValidNsec(ts.Nanoseconds) {
		return nil, unix.EINVAL
	}
	t := unix.Timespec{Sec: ts.Seconds, Nsec: ts.Nanoseconds}
	return []unix.Timespec{t, t}, nil
}

func unixTimevals(ts wsfsprotocol.Timespec) ([]unix.Timeval, error) {
	sec, usec, err := TimevalParts(ts)
	if err != nil {
		return nil, err
	}
	t := unix.Timeval{Sec: sec, Usec: usec}
	return []unix.Timeval{t, t}, nil
}
