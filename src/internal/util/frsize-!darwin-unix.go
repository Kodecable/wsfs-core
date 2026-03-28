//go:build !darwin && unix

package util

import "syscall"

func Frsize(t *syscall.Statfs_t) uint64 {
	return uint64(t.Frsize)
}
