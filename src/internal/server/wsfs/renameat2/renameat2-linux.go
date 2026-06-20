//go:build linux

package renameat2

import (
	"syscall"
	"wsfs-core/internal/share/wsfsunixconv"

	"golang.org/x/sys/unix"
)

func Renameat2(fd1 int, path1 string, fd2 int, path2 string, flag uint32) error {
	if flag & ^wsfsunixconv.AcceptedWSFSRenameFlags != 0 {
		return syscall.ENOTSUP
	}

	var unixFlag uint32
	for protocolFlag, platformFlag := range wsfsunixconv.RenameFlagToUnix {
		if flag&protocolFlag != 0 {
			unixFlag |= platformFlag
		}
	}

	return unix.Renameat2(fd1, path1, fd2, path2, uint(unixFlag))
}
