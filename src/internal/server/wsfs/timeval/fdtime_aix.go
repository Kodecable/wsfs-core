//go:build aix

package timeval

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

func SetFDMTime(fd int, ts wsfsprotocol.Timespec) error {
	return unix.ENOTSUP
}
