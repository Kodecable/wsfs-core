//go:build linux || darwin

package xattr

import (
	"errors"
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

const (
	bufGrowTry  = 3
	bufInitSize = 256
)

func Get(path string, key string, mode uint32) ([]byte, error) {
	if (mode & ^wsfsprotocol.XATTR_NOFOLLOW) != 0 {
		return nil, syscall.EINVAL
	}

	get := unix.Getxattr
	if mode&wsfsprotocol.XATTR_NOFOLLOW != 0 {
		get = unix.Lgetxattr
	}

	buf := make([]byte, bufInitSize)
	for range bufGrowTry {
		size, err := get(path, key, buf)
		if err == nil {
			return buf[:size], nil
		}
		if !errors.Is(err, syscall.ERANGE) {
			return nil, err
		}

		size, err = get(path, key, nil)

		if err != nil {
			return nil, err
		}
		if size < 0 {
			return nil, syscall.EIO
		}
		if size == 0 {
			return []byte{}, nil
		}

		buf = make([]byte, size)
	}
	return nil, syscall.EIO
}

func List(path string, mode uint32) ([]byte, error) {
	if (mode & ^wsfsprotocol.XATTR_NOFOLLOW) != 0 {
		return nil, syscall.EINVAL
	}

	list := unix.Listxattr
	if mode&wsfsprotocol.XATTR_NOFOLLOW != 0 {
		list = unix.Llistxattr
	}

	buf := make([]byte, bufInitSize)
	for range bufGrowTry {
		size, err := list(path, buf)
		if err == nil {
			return buf[:size], nil
		}
		if !errors.Is(err, syscall.ERANGE) {
			return nil, err
		}

		size, err = list(path, nil)

		if err != nil {
			return nil, err
		}
		if size < 0 {
			return nil, syscall.EIO
		}
		if size == 0 {
			return []byte{}, nil
		}

		buf = make([]byte, size)
	}
	return nil, syscall.EIO
}
