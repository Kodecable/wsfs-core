//go:build unix && !darwin

package wsfs

import (
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"
)

func init() {
	errorCodeMap[syscall.EBADFD] = wsfsprotocol.ErrorInvailFD
}
