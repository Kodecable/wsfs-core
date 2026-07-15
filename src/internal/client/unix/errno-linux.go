//go:build linux

package unix

import (
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"
)

func init() {
	errorCodeMap[wsfsprotocol.ErrorNoXAttr] = syscall.ENODATA
}
