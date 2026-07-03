//go:build windows

package windows

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/rs/zerolog/log"
	"github.com/winfsp/cgofuse/fuse"
)

var errorCodeMap = map[uint8]int{
	wsfsprotocol.ErrorOK:                 0,
	wsfsprotocol.ErrorUnknown:            -fuse.EIO,
	wsfsprotocol.ErrorAccessRestricted:   -fuse.EACCES,
	wsfsprotocol.ErrorBusy:               -fuse.EBUSY,
	wsfsprotocol.ErrorExists:             -fuse.EEXIST,
	wsfsprotocol.ErrorTooLong:            -fuse.ENAMETOOLONG,
	wsfsprotocol.ErrorInvalid:            -fuse.EINVAL,
	wsfsprotocol.ErrorInvalidFD:          -fuse.EBADF,
	wsfsprotocol.ErrorNotExists:          -fuse.ENOENT,
	wsfsprotocol.ErrorLoop:               -fuse.ELOOP,
	wsfsprotocol.ErrorNoSpace:            -fuse.ENOSPC,
	wsfsprotocol.ErrorNotEmpty:           -fuse.ENOTEMPTY,
	wsfsprotocol.ErrorType:               -fuse.ENOTDIR,
	wsfsprotocol.ErrorIO:                 -fuse.EIO,
	wsfsprotocol.ErrorNotSupport:         -fuse.ENOTSUP,
	wsfsprotocol.ErrorStateBlocked:       -fuse.EPERM,
	wsfsprotocol.ErrorSpecialFileBlocked: -fuse.EBUSY,
	wsfsprotocol.ErrorCrossDevice:        -fuse.EXDEV,
}

func errnoFromCode(code uint8) int {
	if errno, ok := errorCodeMap[code]; ok {
		return errno
	}
	log.Error().Uint8("Code", code).Msg("Unknown WSFS error code")
	return -fuse.EIO
}
