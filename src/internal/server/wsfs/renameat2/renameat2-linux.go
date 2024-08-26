//go:build linux

package renameat2

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func Renameat2(fd1 int, path1 string, fd2 int, path2 string, flag uint32) error {
	if flag&unix.RENAME_WHITEOUT != 0 {
		return syscall.ENOTSUP
	}
	return unix.Renameat2(fd1, path1, fd2, path2, uint(flag))
}
