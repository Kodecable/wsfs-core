//go:build linux

package wsfsunixconv

import (
	"runtime"

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
	wsfsprotocol.O_DSYNC:     unix.O_DSYNC,
	wsfsprotocol.O_SYNC:      unix.O_SYNC,
	wsfsprotocol.O_NOFOLLOW:  unix.O_NOFOLLOW,
	wsfsprotocol.O_DIRECT:    unix.O_DIRECT,
	wsfsprotocol.O_NOATIME:   unix.O_NOATIME,
}

var IgnoredUnixOpenFlagBits = unix.O_LARGEFILE | unix.O_CLOEXEC | unix.O_NOCTTY | unix.O_NONBLOCK | unix.O_NDELAY

func init() {
	IgnoredUnixOpenFlagBits |= kernelLargeFileOpenFlag()
}

// kernelLargeFileOpenFlag returns the Linux kernel-side O_LARGEFILE bit that
// may appear in FUSE open/create flags. On many 64-bit userspace ABIs,
// unix.O_LARGEFILE is 0 because large-file support is the default, but the
// kernel still forces the real arch UAPI bit into file->f_flags before passing
// flags to FUSE.
func kernelLargeFileOpenFlag() int {
	switch runtime.GOARCH {
	case "amd64", "loong64", "riscv64", "s390x":
		return 0x8000
	case "arm64":
		return 0x20000
	case "mips64", "mips64le":
		return 0x2000
	case "ppc64", "ppc64le":
		return 0x10000
	case "sparc64":
		return 0x40000
	default:
		return 0
	}
}
