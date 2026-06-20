//go:build linux

package ofdlock

import (
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/share/wsfsunixconv"

	"golang.org/x/sys/unix"
)

// At golang.org/x/sys/unix v0.46.0:
// unix.FcntlFlock available on: linux, darwin, aix, openbsd, zos, solaris
// unix.F_OFD_* available on: linux, solaris
// TRACK: https://github.com/golang/go/issues/73351 (x/sys/unix: F_OFD_* consts are not present on Darwin)

func lockToSys(in wsfsprotocol.FileLockInfo) (out syscall.Flock_t, ok bool) {
	out.Type, ok = wsfsunixconv.LockTypeToUnix[in.Type]
	if !ok {
		return
	}
	if in.Type != wsfsprotocol.FILELOCK_UNLOCK {
		out.Len = int64(in.Size)
		out.Start = int64(in.Start)
		out.Whence = int16(wsfsunixconv.WhenceToUnix[in.Whence])
	}
	return
}

func lockFromSys(in *syscall.Flock_t) (out wsfsprotocol.FileLockInfo) {
	out.Type = wsfsunixconv.LockTypeFromUnix[in.Type]
	if out.Type != wsfsprotocol.FILELOCK_UNLOCK {
		out.Size = uint64(in.Len)
		out.Start = uint64(in.Start)
		out.Whence = wsfsunixconv.WhenceFromUnix[int(in.Whence)]
	}
	return
}

func GetLock(fd int, lock wsfsprotocol.FileLockInfo) (result wsfsprotocol.FileLockInfo, err error) {
	flock, ok := lockToSys(lock)
	if !ok {
		err = syscall.EINVAL
		return
	}

	if err = syscall.FcntlFlock(uintptr(fd), unix.F_OFD_GETLK, &flock); err != nil {
		return
	}

	return lockFromSys(&flock), nil
}

func SetLock(fd int, lock wsfsprotocol.FileLockInfo, blocking bool) error {
	cmd := unix.F_OFD_SETLK
	if blocking {
		cmd = unix.F_OFD_SETLKW
	}

	flock, ok := lockToSys(lock)
	if !ok {
		return syscall.EINVAL
	}

	return syscall.FcntlFlock(uintptr(fd), int(cmd), &flock)
}
