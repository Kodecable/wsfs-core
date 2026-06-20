//go:build linux

package wsfs

import (
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"
)

func init() {
	errorCodeMap[syscall.EBADFD] = wsfsprotocol.ErrorIO
}
