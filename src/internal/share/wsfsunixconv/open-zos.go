//go:build zos

package wsfsunixconv

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

var OpenFlagToUnix = map[uint32]int{
	wsfsprotocol.O_RDONLY:    unix.O_RDONLY,
	wsfsprotocol.O_WRONLY:    unix.O_WRONLY,
	wsfsprotocol.O_RDWR:      unix.O_RDWR,
	wsfsprotocol.O_TRUNC:     unix.O_TRUNC,
	wsfsprotocol.O_EXCL:      unix.O_EXCL,
	wsfsprotocol.O_CREAT:     unix.O_CREAT,
	wsfsprotocol.O_DIRECTORY: unix.O_DIRECTORY,
	wsfsprotocol.O_APPEND:    unix.O_APPEND,
	wsfsprotocol.O_SYNC:      unix.O_SYNC,
	wsfsprotocol.O_NOFOLLOW:  unix.O_NOFOLLOW,
	wsfsprotocol.O_DIRECT:    unix.O_DIRECT,
}

var IgnoredUnixOpenFlagBits = unix.O_LARGEFILE | unix.O_CLOEXEC | unix.O_NOCTTY | unix.O_NONBLOCK | unix.O_NDELAY
