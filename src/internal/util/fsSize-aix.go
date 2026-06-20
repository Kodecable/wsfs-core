//go:build aix

package util

import "syscall"

func FsSize(fspath string) (total, free, avail uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(fspath, &stat)
	blockSize := stat.Fsize
	if blockSize == 0 {
		blockSize = stat.Bsize
	}
	return stat.Blocks * blockSize, stat.Bfree * blockSize, stat.Bavail * blockSize, err
}
