//go:build dragonfly

package util

import "syscall"

func FsSize(fspath string) (total, free, avail uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(fspath, &stat)
	return uint64(stat.Blocks * stat.Bsize), uint64(stat.Bfree * stat.Bsize), uint64(stat.Bavail * stat.Bsize), err
}
