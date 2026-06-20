//go:build netbsd || solaris

package util

import (
	"golang.org/x/sys/unix"
)

func FsSize(fspath string) (total, free, avail uint64, err error) {
	var stat unix.Statvfs_t
	err = unix.Statvfs(fspath, &stat)
	return stat.Blocks * stat.Frsize, stat.Bfree * stat.Frsize, stat.Bavail * stat.Frsize, err
}
