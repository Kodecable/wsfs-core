//go:build !darwin && unix

package frsize

import "syscall"

func Frsize(t *syscall.Statfs_t) int64 {
	return int64(t.Frsize)
}
