//go:build unix

package util

import "syscall"

func FsSize(fspath string) (total, free, avail uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(fspath, &stat)
	frSize := Frsize(&stat)
	return stat.Blocks * frSize, stat.Bfree * frSize, stat.Bavail * frSize, err
}
