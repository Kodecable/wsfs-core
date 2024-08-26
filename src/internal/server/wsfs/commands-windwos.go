//go:build windows

package wsfs

import (
	"syscall"
	"unsafe"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"
)

var kernel32Dll = syscall.MustLoadDLL("kernel32.dll")
var getDiskFreeSpaceExW = kernel32Dll.MustFindProc("GetDiskFreeSpaceExW")

func (s *session) cmdFsStat(clientMark uint8, writeCh chan<- *util.Buffer, lpath string) {
	defer s.wg.Done()

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	str, _ := syscall.UTF16PtrFromString(apath)
	var freeBytesAvailableToCaller int64
	var totalNumberOfBytes int64
	var totalNumberOfFreeBytes int64

	getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(str)),
		uintptr(unsafe.Pointer(&freeBytesAvailableToCaller)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)

	//fake data
	writeCh <- msg(clientMark, wsfsprotocol.ErrorOK,
		uint64(totalNumberOfBytes),
		uint64(totalNumberOfFreeBytes),
		uint64(freeBytesAvailableToCaller),
	)
}
