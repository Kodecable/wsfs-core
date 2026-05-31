//go:build !linux

package renameat2

import "syscall"

func Renameat2(_ int, _ string, _ int, _ string, _ uint32) error {
	return syscall.ENOTSUP
}
