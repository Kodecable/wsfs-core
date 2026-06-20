//go:build unix

package wsfsunixconv

import (
	"golang.org/x/sys/unix"
	"wsfs-core/internal/share/wsfsprotocol"
)

var LockTypeToUnix = map[uint8]int16{
	wsfsprotocol.FILELOCK_READLOCK:  unix.F_RDLCK,
	wsfsprotocol.FILELOCK_UNLOCK:    unix.F_UNLCK,
	wsfsprotocol.FILELOCK_WRITELOCK: unix.F_WRLCK,
}

var LockTypeFromUnix = map[int16]uint8{}

func init() {
	for protocol, platform := range LockTypeToUnix {
		LockTypeFromUnix[platform] = protocol
	}
}
