//go:build unix

package unix

import (
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"
)

var errorCodeMap = map[uint8]syscall.Errno{
	wsfsprotocol.ErrorOK:         syscall.Errno(0),
	wsfsprotocol.ErrorAccess:     syscall.EACCES,
	wsfsprotocol.ErrorBusy:       syscall.EBUSY,
	wsfsprotocol.ErrorExists:     syscall.EEXIST,
	wsfsprotocol.ErrorTooLoong:   syscall.ENAMETOOLONG,
	wsfsprotocol.ErrorInvail:     syscall.EINVAL,
	wsfsprotocol.ErrorInvailFD:   syscall.EBADFD,
	wsfsprotocol.ErrorNotExists:  syscall.ENOENT,
	wsfsprotocol.ErrorLoop:       syscall.ELOOP,
	wsfsprotocol.ErrorNoSpace:    syscall.ENOSPC,
	wsfsprotocol.ErrorNotEmpty:   syscall.ENOTEMPTY,
	wsfsprotocol.ErrorType:       syscall.ENOTDIR,
	wsfsprotocol.ErrorIO:         syscall.EIO,
	wsfsprotocol.ErrorNotSupport: syscall.ENOTSUP,
	//syscall.EMLINK:       wsfsprotocol.ErrorLinkMax,
}
