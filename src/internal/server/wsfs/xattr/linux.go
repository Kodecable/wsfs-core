//go:build linux

package xattr

import (
	"errors"
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

func Set(path string, key string, value []byte, mode uint32) error {
	nofollow := mode&wsfsprotocol.XATTR_NOFOLLOW != 0
	switch mode & ^wsfsprotocol.XATTR_NOFOLLOW {
	case wsfsprotocol.SETXATTR_NORMAL:
		return set(path, key, value, 0, nofollow)
	case wsfsprotocol.SETXATTR_APPEND:
		oldValue, err := Get(path, key, mode&wsfsprotocol.XATTR_NOFOLLOW)
		if err != nil {
			return err
		}
		return set(path, key, append(oldValue, value...), 0, nofollow)
	case wsfsprotocol.SETXATTR_CREATE:
		return set(path, key, value, unix.XATTR_CREATE, nofollow)
	case wsfsprotocol.SETXATTR_REPLACE:
		return set(path, key, value, unix.XATTR_REPLACE, nofollow)
	default:
		return syscall.EINVAL
	}
}

func Remove(path string, key string, mode uint32) error {
	if mode&^wsfsprotocol.XATTR_NOFOLLOW != 0 {
		return syscall.EINVAL
	}
	if mode&wsfsprotocol.XATTR_NOFOLLOW != 0 {
		return unix.Lremovexattr(path, key)
	}
	return unix.Removexattr(path, key)
}

func IsNoXAttr(err error) bool {
	return errors.Is(err, unix.ENODATA)
}

func set(path string, key string, value []byte, flags int, nofollow bool) error {
	if nofollow {
		return unix.Lsetxattr(path, key, value, flags)
	}
	return unix.Setxattr(path, key, value, flags)
}
