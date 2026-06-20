//go:build linux

package reflink

import "golang.org/x/sys/unix"

func CloneFileRange(dstFD int, srcFD int, dstOffset uint64, srcOffset uint64, size uint64) error {
	if dstOffset == 0 && srcOffset == 0 && size == 0 {
		return unix.IoctlFileClone(dstFD, srcFD)
	}

	cloneRange := unix.FileCloneRange{
		Src_fd:      int64(srcFD),
		Src_offset:  srcOffset,
		Src_length:  size,
		Dest_offset: dstOffset,
	}
	return unix.IoctlFileCloneRange(dstFD, &cloneRange)
}
