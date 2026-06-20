//go:build darwin

package util

import "syscall"

func FsSize(fspath string) (total, free, avail uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(fspath, &stat)
	return stat.Blocks * uint64(stat.Bsize), stat.Bfree * uint64(stat.Bsize), stat.Bavail * uint64(stat.Bsize), err
}
