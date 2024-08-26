//go:build darwin

package fallocate

import "syscall"

func Fallocate(_ int, _ uint32, _ int64, _ int64) error {
	return syscall.ENOTSUP
}
