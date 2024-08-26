//go:build !darwin && unix

package fallocate

import "syscall"

func Fallocate(fd int, mode uint32, off int64, len int64) (err error) {
	return syscall.Fallocate(fd, mode, off, len)
}
