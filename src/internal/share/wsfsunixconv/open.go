//go:build unix

package wsfsunixconv

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

var (
	OpenFlagFromUnix        = map[int]uint32{}
	AcceptdUnixOpenFlagBits int
	AcceptdWSFSOpenFlagBits uint32
)

func init() {
	OpenFlagFromUnix[unix.O_RDONLY] = wsfsprotocol.O_RDONLY
	OpenFlagFromUnix[unix.O_WRONLY] = wsfsprotocol.O_WRONLY
	OpenFlagFromUnix[unix.O_RDWR] = wsfsprotocol.O_RDWR
	AcceptdUnixOpenFlagBits |= unix.O_ACCMODE
	AcceptdWSFSOpenFlagBits |= wsfsprotocol.O_ACCMODE

	for protocol, platform := range OpenFlagToUnix {
		if protocol&wsfsprotocol.O_ACCMODE != 0 {
			continue
		}
		OpenFlagFromUnix[platform] = protocol
		AcceptdUnixOpenFlagBits |= platform
		AcceptdWSFSOpenFlagBits |= protocol
	}
}
