//go:build linux && (386 || arm || mips || mipsle || ppc)

package timeval

import (
	"io/fs"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"

	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

type kernelTimespec64 struct {
	Sec  int64
	Nsec int64
}

func MTimeFromFileInfo(fi fs.FileInfo) wsfsprotocol.Timespec {
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		return wsfsprotocol.Timespec{
			Seconds:     int64(stat.Mtim.Sec),
			Nanoseconds: int64(stat.Mtim.Nsec),
		}
	}
	return FromFileInfo(fi)
}

func Stat(path string, followSymlink bool) (fs.FileInfo, wsfsprotocol.Timespec, error) {
	var flags int
	if !followSymlink {
		flags = unix.AT_SYMLINK_NOFOLLOW
	}
	fi, err := StatFallback(path, followSymlink)
	if err != nil {
		return nil, wsfsprotocol.Timespec{}, err
	}

	var stat unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, flags, unix.STATX_MTIME, &stat); err != nil {
		return fi, MTimeFromFileInfo(fi), nil
	}
	return fi, wsfsprotocol.Timespec{
		Seconds:     stat.Mtime.Sec,
		Nanoseconds: int64(stat.Mtime.Nsec),
	}, nil
}

func SetPathMTime(path string, ts wsfsprotocol.Timespec) error {
	err := utimensatTime64(unix.AT_FDCWD, path, ts, 0)
	if err == unix.ENOSYS {
		times, convErr := unixTimespecs(ts)
		if convErr != nil {
			return convErr
		}
		return unix.UtimesNanoAt(unix.AT_FDCWD, path, times, 0)
	}
	return err
}

func SetFDMTime(fd int, ts wsfsprotocol.Timespec) error {
	fdPath := filepath.Join("/proc/self/fd", strconv.Itoa(fd))
	err := utimensatTime64(unix.AT_FDCWD, fdPath, ts, 0)
	if err == unix.ENOSYS {
		times, convErr := unixTimespecs(ts)
		if convErr != nil {
			return convErr
		}
		return unix.UtimesNanoAt(unix.AT_FDCWD, fdPath, times, 0)
	}
	return err
}

func utimensatTime64(dirfd int, path string, ts wsfsprotocol.Timespec, flags int) error {
	times := [2]kernelTimespec64{
		{Sec: ts.Seconds, Nsec: ts.Nanoseconds},
		{Sec: ts.Seconds, Nsec: ts.Nanoseconds},
	}
	pathp, err := unix.BytePtrFromString(path)
	if err != nil {
		return err
	}
	_, _, errno := unix.Syscall6(
		unix.SYS_UTIMENSAT_TIME64,
		uintptr(dirfd),
		uintptr(unsafe.Pointer(pathp)),
		uintptr(unsafe.Pointer(&times[0])),
		uintptr(flags),
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}
