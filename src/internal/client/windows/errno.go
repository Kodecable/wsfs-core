//go:build windows

package windows

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/winfsp/cgofuse/fuse"
)

var errorCodeMap = map[uint8]int{
	wsfsprotocol.ErrorOK:         0,
	wsfsprotocol.ErrorAccess:     -fuse.EACCES,
	wsfsprotocol.ErrorBusy:       -fuse.EBUSY,
	wsfsprotocol.ErrorExists:     -fuse.EEXIST,
	wsfsprotocol.ErrorTooLoong:   -fuse.ENAMETOOLONG,
	wsfsprotocol.ErrorInvail:     -fuse.EINVAL,
	wsfsprotocol.ErrorInvailFD:   -fuse.EBADF,
	wsfsprotocol.ErrorNotExists:  -fuse.ENOENT,
	wsfsprotocol.ErrorLoop:       -fuse.ELOOP,
	wsfsprotocol.ErrorNoSpace:    -fuse.ENOSPC,
	wsfsprotocol.ErrorNotEmpty:   -fuse.ENOTEMPTY,
	wsfsprotocol.ErrorType:       -fuse.ENOTDIR,
	wsfsprotocol.ErrorIO:         -fuse.EIO,
	wsfsprotocol.ErrorNotSupport: -fuse.ENOTSUP,
}
