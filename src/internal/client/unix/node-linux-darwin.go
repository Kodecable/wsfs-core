//go:build linux || darwin

package unix

import (
	"context"
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"golang.org/x/sys/unix"
)

var _ = (fusefs.NodeGetxattrer)((*fsNode)(nil))

func (n *fsNode) Getxattr(_ context.Context, key string, dest []byte) (uint32, syscall.Errno) {
	data, code := n.fsdata.session.CmdGetXAttr(n.path(), key, wsfsprotocol.XATTR_NOFOLLOW)
	if code != wsfsprotocol.ErrorOK {
		return 0, errnoFromCode(code)
	}
	if len(dest) < len(data) {
		return uint32(len(data)), syscall.ERANGE
	}
	copy(dest, data)
	return uint32(len(data)), 0
}

var _ = (fusefs.NodeSetxattrer)((*fsNode)(nil))

func (n *fsNode) Setxattr(_ context.Context, key string, data []byte, flags uint32) syscall.Errno {
	mode := wsfsprotocol.XATTR_NOFOLLOW
	switch flags {
	case 0:
	case unix.XATTR_CREATE:
		mode |= wsfsprotocol.SETXATTR_CREATE
	case unix.XATTR_REPLACE:
		mode |= wsfsprotocol.SETXATTR_REPLACE
	default:
		return syscall.EINVAL
	}
	return errnoFromCode(n.fsdata.session.CmdSetXAttr(n.path(), key, data, mode))
}

var _ = (fusefs.NodeRemovexattrer)((*fsNode)(nil))

func (n *fsNode) Removexattr(_ context.Context, key string) syscall.Errno {
	return errnoFromCode(n.fsdata.session.CmdRemoveXAttr(n.path(), key, wsfsprotocol.XATTR_NOFOLLOW))
}

var _ = (fusefs.NodeListxattrer)((*fsNode)(nil))

func (n *fsNode) Listxattr(_ context.Context, dest []byte) (uint32, syscall.Errno) {
	data, code := n.fsdata.session.CmdListXAttr(n.path(), wsfsprotocol.XATTR_NOFOLLOW)
	if code != wsfsprotocol.ErrorOK {
		return 0, errnoFromCode(code)
	}
	if len(dest) < len(data) {
		return uint32(len(data)), syscall.ERANGE
	}
	copy(dest, data)
	return uint32(len(data)), 0
}
