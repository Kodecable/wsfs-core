//go:build !linux

package ofdlock

import (
	"syscall"
	"wsfs-core/internal/share/wsfsprotocol"
)

func GetLock(_ int, _ wsfsprotocol.FileLockInfo) (wsfsprotocol.FileLockInfo, error) {
	return wsfsprotocol.FileLockInfo{}, syscall.ENOTSUP
}

func SetLock(_ int, _ wsfsprotocol.FileLockInfo, _ bool) error {
	return syscall.ENOTSUP
}
