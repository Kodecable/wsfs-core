//go:build freebsd

package util

import "syscall"

func FsSize(fspath string) (total, free, avail uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(fspath, &stat)
	if stat.Bavail < 0 {
		stat.Bavail = 0
	}
	return stat.Blocks * stat.Bsize, stat.Bfree * stat.Bsize, uint64(stat.Bavail) * stat.Bsize, err
}
