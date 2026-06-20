//go:build openbsd

package util

import "syscall"

func FsSize(fspath string) (total, free, avail uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(fspath, &stat)
	if stat.F_bavail < 0 {
		stat.F_bavail = 0
	}
	return stat.F_blocks * uint64(stat.F_bsize), stat.F_bfree * uint64(stat.F_bsize), uint64(stat.F_bavail) * uint64(stat.F_bsize), err
}
