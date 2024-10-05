//go:build windows

package wsfs

import (
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"

	"golang.org/x/sys/windows"
)

func (s *session) cmdFsStat(clientMark uint8, writeCh chan<- *util.Buffer, lpath string) {
	defer s.wg.Done()

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	str, _ := windows.UTF16PtrFromString(apath)
	var free, total, avail uint64
	windows.GetDiskFreeSpaceEx(str, &free, &total, &avail)

	//fake data
	writeCh <- msg(clientMark, wsfsprotocol.ErrorOK, free, total, avail)
}
