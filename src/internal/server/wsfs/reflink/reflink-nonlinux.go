//go:build !linux

package reflink

import "syscall"

func CloneFileRange(_ int, _ int, _ uint64, _ uint64, _ uint64) error {
	return syscall.ENOTSUP
}
