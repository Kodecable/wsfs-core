//go:build linux

package unix

import (
	"context"
	"syscall"
	"unsafe"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/share/wsfsunixconv"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"golang.org/x/sys/unix"
)

func decodeCloneFileRequest(cmd uint32, input []byte) (srcFD uint32, srcOffset uint64, dstOffset uint64, size uint64, errno syscall.Errno) {
	switch cmd {
	case unix.FICLONE:
		if len(input) < int(unsafe.Sizeof(int32(0))) {
			return 0, 0, 0, 0, syscall.EINVAL
		}
		src := *(*int32)(unsafe.Pointer(&input[0]))
		if src < 0 {
			return 0, 0, 0, 0, syscall.EBADF
		}
		return uint32(src), 0, 0, 0, 0
	case unix.FICLONERANGE:
		if len(input) < int(unsafe.Sizeof(unix.FileCloneRange{})) {
			return 0, 0, 0, 0, syscall.EINVAL
		}
		cloneRange := *(*unix.FileCloneRange)(unsafe.Pointer(&input[0]))
		if cloneRange.Src_fd < 0 {
			return 0, 0, 0, 0, syscall.EBADF
		}
		return uint32(cloneRange.Src_fd), cloneRange.Src_offset, cloneRange.Dest_offset, cloneRange.Src_length, 0
	default:
		return 0, 0, 0, 0, syscall.ENOTTY
	}
}

var _ = (fusefs.NodeIoctler)((*fsNode)(nil))

func (n *fsNode) Ioctl(_ context.Context, f fusefs.FileHandle, cmd uint32, arg uint64, input []byte, _ []byte) (int32, syscall.Errno) {
	fd, ok := f.(uint32)
	if !ok {
		return 0, syscall.EBADF
	}
	srcFD, srcOffset, dstOffset, size, errno := decodeCloneFileRequest(cmd, input)
	if errno != 0 {
		return 0, errno
	}
	_ = arg
	code := n.fsdata.session.CmdCloneFileRange(srcFD, fd, srcOffset, dstOffset, size)
	return 0, errnoFromCode(code)
}

func fileLockToProtocol(fileLock *fuse.FileLock) (wsfsprotocol.FileLockInfo, bool) {
	lockType, ok := wsfsunixconv.LockTypeFromUnix[int16(fileLock.Typ)]
	if !ok {
		return wsfsprotocol.FileLockInfo{}, false
	}
	lockLength := uint64(0)
	if fileLock.End != (1<<63)-1 {
		lockLength = fileLock.End - fileLock.Start + 1
	}
	return wsfsprotocol.FileLockInfo{Start: fileLock.Start, Size: lockLength, Type: lockType}, true
}

func wholeFileLockToProtocol(fileLock *fuse.FileLock) (wsfsprotocol.FileLockInfo, bool) {
	lockType, ok := wsfsunixconv.LockTypeFromUnix[int16(fileLock.Typ)]
	if !ok {
		return wsfsprotocol.FileLockInfo{}, false
	}
	return wsfsprotocol.FileLockInfo{
		Type:   lockType,
		Whence: wsfsprotocol.WHENCE_SET,
		Start:  0,
		Size:   0,
	}, true
}

func fileLockFromProtocol(dst *fuse.FileLock, fileLock wsfsprotocol.FileLockInfo) bool {
	lockType, ok := wsfsunixconv.LockTypeToUnix[fileLock.Type]
	if !ok {
		return false
	}
	dst.Start = fileLock.Start
	if fileLock.Size == 0 {
		dst.End = (1 << 63) - 1
	} else {
		dst.End = fileLock.Start + fileLock.Size - 1
	}
	dst.Typ = uint32(lockType)
	dst.Pid = 0
	return true
}

func markNoLock(out *fuse.FileLock) {
	*out = fuse.FileLock{Typ: uint32(syscall.F_UNLCK)}
}

var _ = (fusefs.NodeGetlker)((*fsNode)(nil))

func (n *fsNode) Getlk(_ context.Context, f fusefs.FileHandle, _ uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) syscall.Errno {
	fd, ok := f.(uint32)
	if !ok {
		return syscall.EBADF
	}
	if flags&fuse.FUSE_LK_FLOCK != 0 {
		switch n.fsdata.flockMode {
		case session.FlockModeNoop:
			markNoLock(out)
			return 0
		case session.FlockModeOFD:
			wsfsLock, ok := wholeFileLockToProtocol(lk)
			if !ok {
				return syscall.EINVAL
			}
			outLk, code := n.fsdata.session.CmdGetFileLock(fd, wsfsLock)
			if code != wsfsprotocol.ErrorOK {
				return errnoFromCode(code)
			}
			if !fileLockFromProtocol(out, outLk) {
				return syscall.EINVAL
			}
			return 0
		case session.FlockModeUnsupported:
			return syscall.ENOTSUP
		default:
			return syscall.ENOTSUP
		}
	}
	wsfsLock, ok := fileLockToProtocol(lk)
	if !ok {
		return syscall.EINVAL
	}
	outLk, code := n.fsdata.session.CmdGetFileLock(fd, wsfsLock)
	if code != wsfsprotocol.ErrorOK {
		return errnoFromCode(code)
	}
	if !fileLockFromProtocol(out, outLk) {
		return syscall.EINVAL
	}
	return 0
}

var _ = (fusefs.NodeSetlker)((*fsNode)(nil))

func (n *fsNode) Setlk(_ context.Context, f fusefs.FileHandle, _ uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	fd, ok := f.(uint32)
	if !ok {
		return syscall.EBADF
	}
	if flags&fuse.FUSE_LK_FLOCK != 0 {
		switch n.fsdata.flockMode {
		case session.FlockModeNoop:
			return 0
		case session.FlockModeOFD:
			wsfsLock, ok := wholeFileLockToProtocol(lk)
			if !ok {
				return syscall.EINVAL
			}
			return errnoFromCode(n.fsdata.session.CmdSetFileLock(fd, wsfsLock))
		case session.FlockModeUnsupported:
			return syscall.ENOTSUP
		default:
			return syscall.ENOTSUP
		}
	}
	wsfsLock, ok := fileLockToProtocol(lk)
	if !ok {
		return syscall.EINVAL
	}
	return errnoFromCode(n.fsdata.session.CmdSetFileLock(fd, wsfsLock))
}

var _ = (fusefs.NodeSetlkwer)((*fsNode)(nil))

func (n *fsNode) Setlkw(_ context.Context, f fusefs.FileHandle, _ uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	fd, ok := f.(uint32)
	if !ok {
		return syscall.EBADF
	}
	if flags&fuse.FUSE_LK_FLOCK != 0 {
		switch n.fsdata.flockMode {
		case session.FlockModeNoop:
			return 0
		case session.FlockModeOFD:
			wsfsLock, ok := wholeFileLockToProtocol(lk)
			if !ok {
				return syscall.EINVAL
			}
			return errnoFromCode(n.fsdata.session.CmdSetFileLockWait(fd, wsfsLock))
		case session.FlockModeUnsupported:
			return syscall.ENOTSUP
		default:
			return syscall.ENOTSUP
		}
	}
	wsfsLock, ok := fileLockToProtocol(lk)
	if !ok {
		return syscall.EINVAL
	}
	return errnoFromCode(n.fsdata.session.CmdSetFileLockWait(fd, wsfsLock))
}
