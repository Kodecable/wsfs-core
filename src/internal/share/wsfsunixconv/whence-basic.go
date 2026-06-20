//go:build unix && !(linux || freebsd || darwin || aix || dragonfly || illumos || netbsd || solaris)

package wsfsunixconv

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

var WhenceToUnix = map[uint8]int{
	wsfsprotocol.WHENCE_SET: unix.SEEK_SET,
	wsfsprotocol.WHENCE_CUR: unix.SEEK_CUR,
	wsfsprotocol.WHENCE_END: unix.SEEK_END,
}
