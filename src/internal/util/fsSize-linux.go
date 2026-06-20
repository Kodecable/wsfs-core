//go:build linux

package util

import "syscall"

func FsSize(fspath string) (total, free, avail uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(fspath, &stat)
	return stat.Blocks * uint64(stat.Frsize), stat.Bfree * uint64(stat.Frsize), stat.Bavail * uint64(stat.Frsize), err
}
