//go:build unix

package timeval

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

func ValidNsec(nsec int64) bool {
	return nsec >= 0 && nsec < 1_000_000_000
}

func FitsInt32(v int64) bool {
	return int64(int32(v)) == v
}

func TimevalParts(ts wsfsprotocol.Timespec) (int64, int64, error) {
	if !ValidNsec(ts.Nanoseconds) {
		return 0, 0, unix.EINVAL
	}
	sec := ts.Seconds
	usec := (ts.Nanoseconds + 999) / 1000
	if usec >= 1_000_000 {
		if sec == int64(^uint64(0)>>1) {
			return 0, 0, unix.ERANGE
		}
		sec++
		usec -= 1_000_000
	}
	return sec, usec, nil
}
