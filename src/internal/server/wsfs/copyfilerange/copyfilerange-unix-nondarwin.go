//go:build !darwin && unix

package copyfilerange

import (
	"golang.org/x/sys/unix"
)

func CopyFileRange(rfd int, roff *int64, wfd int, woff *int64, len int, flags int) (n int, err error) {
	return unix.CopyFileRange(rfd, roff, wfd, woff, len, flags)
}
