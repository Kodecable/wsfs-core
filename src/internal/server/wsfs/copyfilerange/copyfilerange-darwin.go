//go:build darwin

package copyfilerange

import "syscall"

func CopyFileRange(_ int, _ *int64, _ int, _ *int64, _ int, _ int) (int, error) {
	return 0, syscall.ENOTSUP
}
