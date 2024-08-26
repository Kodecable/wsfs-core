//go:build !unix

package wsfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"
)

type sfd_t *os.File

func (s *session) convOwner(_ os.FileInfo) (ownerInfo uint8) {
	return wsfsprotocol.OWNER_UG
}

func (s *session) cmdOpen(clientMark uint8, writeCh chan<- *util.Buffer, path string, oflag uint32, fmode uint32) {
	defer s.wg.Done()

	if !util.IsUrlValid(path) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + path

	sfd, err := os.OpenFile(apath, int(oflag), fs.FileMode(fmode))

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		wfd := s.newFD(sfd_t(sfd))
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK, wfd)
	}
}

func (s *session) cmdClose(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	// TODO: Carefully handle EINTR
	// when close() return EINTR. Linux and AIX typically close the file
	// descriptor despite interruption, whereas HPUX may keep the descriptor
	// open.
	s.fds.Delete(wfd)
	err := (*os.File)(sfd).Close()

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func readAndSend(clientMark uint8, writeCh chan<- *util.Buffer, fd *os.File, size uint64, okCode uint8) bool {
	buf := bufPool.Get().(*util.Buffer)
	buf.Put(clientMark)
	buf.Put(okCode)
	readed, err := fd.Read(buf.DirectPutStart(int(size)))
	buf.DirectPutDone(readed)

	if err != nil {
		buf.Done()
		buf.Put(clientMark)
		buf.Put(osErrCode(err))
		buf.Put("syscall error")
		writeCh <- buf
		return false
	} else {
		writeCh <- buf
		return true
	}
}

func (s *session) cmdRead(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32, size uint64) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	if size < dataPerMsg {
		readAndSend(clientMark, writeCh, (*os.File)(sfd), size, wsfsprotocol.ErrorOK)
	} else {
		for range size / dataPerMsg {
			ok := readAndSend(clientMark, writeCh, (*os.File)(sfd), dataPerMsg, wsfsprotocol.ErrorPartialResponse)
			if !ok {
				return
			}
		}
		if size%dataPerMsg == 0 {
			writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
		} else {
			readAndSend(clientMark, writeCh, (*os.File)(sfd), size%dataPerMsg, wsfsprotocol.ErrorOK)
		}
	}
}

func (s *session) cmdReadLink(clientMark uint8, writeCh chan<- *util.Buffer, lpath string) {
	defer s.wg.Done()

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	target, err := os.Readlink(apath)
	if err != nil {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorType, "syscall error")
	}

	if target[0] != '/' {
		target = filepath.Clean(filepath.Join(apath, target))
	}

	if strings.HasPrefix(target, s.storage.Path) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK, target)
	} else {
		// we will handle this kind symlinks in getattr and readdir to
		// prevent escape storage root. So, fake it.
		writeCh <- msg(clientMark, wsfsprotocol.ErrorType, "syscall error")
	}
}

func (s *session) cmdSeek(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32, flag uint8, off int64) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	whence := int(flag)
	if whence > 2 {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
	}

	offset, err := (*os.File)(sfd).Seek(off, whence)

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK, uint64(offset))
	}
}

func (s *session) cmdWrite(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32, data *util.Buffer) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	count, err := (*os.File)(sfd).Write(data.Done())
	bufPool.Put(data)

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK, uint64(count))
	}
}

func (s *session) cmdAllocate(clientMark uint8, writeCh chan<- *util.Buffer, _ uint32, _ uint32, _ uint64, _ uint64) {
	defer s.wg.Done()
	writeCh <- msg(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
}

func (s *session) cmdSetAttr(clientMark uint8, writeCh chan<- *util.Buffer, lpath string, flag uint8, size uint64, _ int64, _ uint32, _ uint8) {
	defer s.wg.Done()

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	if flag&wsfsprotocol.SETATTR_SIZE != 0 {
		err := os.Truncate(apath, int64(size))
		if err != nil {
			writeCh <- msg(clientMark, osErrCode(err), "syscall error")
		}
	}
	if flag&wsfsprotocol.SETATTR_MTIME != 0 {
		// not support
	}
	if flag&wsfsprotocol.SETATTR_MODE != 0 {
		// not support
	}
	if flag&wsfsprotocol.SETATTR_OWNER != 0 {
		// not support
	}
	writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
}

func (s *session) cmdSync(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	err := (*os.File)(sfd).Sync()

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func (s *session) cmdMkdir(clientMark uint8, writeCh chan<- *util.Buffer, lpath string, mode uint32) {
	defer s.wg.Done()

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	err := os.Mkdir(apath, fs.FileMode(mode))

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func (s *session) cmdSymLink(clientMark uint8, writeCh chan<- *util.Buffer, oldPath string, newPath string) {
	defer s.wg.Done()

	if !util.IsUrlValid(oldPath) || !util.IsUrlValid(newPath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	aOldPath := s.storage.Path + oldPath
	aNewPath := s.storage.Path + newPath

	err := os.Symlink(aOldPath, aNewPath)

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func (s *session) cmdRemove(clientMark uint8, writeCh chan<- *util.Buffer, lpath string) {
	defer s.wg.Done()

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	err := os.Remove(apath)

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func (s *session) cmdRmDir(clientMark uint8, writeCh chan<- *util.Buffer, lpath string) {
	defer s.wg.Done()

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	err := os.Remove(apath)

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func readAtAndSend(clientMark uint8, writeCh chan<- *util.Buffer, fd *os.File, off uint64, size uint64, okCode uint8) (uint64, bool) {
	buf := bufPool.Get().(*util.Buffer)
	buf.Put(clientMark)
	buf.Put(okCode)
	readed, err := fd.ReadAt(buf.DirectPutStart(int(size)), int64(off))
	buf.DirectPutDone(readed)

	if err != nil {
		buf.Done()
		buf.Put(clientMark)
		buf.Put(osErrCode(err))
		buf.Put("syscall error")
		writeCh <- buf
		return 0, false
	} else {
		writeCh <- buf
		return uint64(readed), true
	}
}

func (s *session) cmdReadAt(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32, off uint64, size uint64) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	if size < dataPerMsg {
		readAtAndSend(clientMark, writeCh, (*os.File)(sfd), off, size, wsfsprotocol.ErrorOK)
	} else {
		for range size / dataPerMsg {
			readed, ok := readAtAndSend(clientMark, writeCh, (*os.File)(sfd), off, dataPerMsg, wsfsprotocol.ErrorPartialResponse)
			if !ok {
				return
			}
			off += readed
		}
		if size%dataPerMsg == 0 {
			writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
		} else {
			readAtAndSend(clientMark, writeCh, (*os.File)(sfd), off, size%dataPerMsg, wsfsprotocol.ErrorOK)
		}
	}
}

func (s *session) cmdWriteAt(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32, off uint64, data *util.Buffer) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	count, err := (*os.File)(sfd).WriteAt(data.Done(), int64(off))
	bufPool.Put(data)

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK, uint64(count))
	}
}

func (s *session) cmdCopyFileRange(clientMark uint8, writeCh chan<- *util.Buffer, _ uint32, _ uint32, _ uint64, _ uint64, _ uint64) {
	defer s.wg.Done()
	writeCh <- msg(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
}

func (s *session) cmdRename(clientMark uint8, writeCh chan<- *util.Buffer, oldPath string, newPath string, flag uint32) {
	defer s.wg.Done()

	if !util.IsUrlValid(oldPath) || !util.IsUrlValid(newPath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	aOldPath := s.storage.Path + oldPath
	aNewPath := s.storage.Path + newPath

	var err error
	if flag == 0 {
		err = os.Rename(aOldPath, aNewPath)
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
	}

	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func (s *session) cmdSetAttrByFD(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32, flag uint8, size uint64, _ int64, _ uint32, _ uint8) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	if flag&wsfsprotocol.SETATTR_SIZE != 0 {
		err := (*os.File)(sfd).Truncate(int64(size))
		if err != nil {
			writeCh <- msg(clientMark, osErrCode(err), "syscall error")
		}
	}
	if flag&wsfsprotocol.SETATTR_MTIME != 0 {
		// not support
	}
	if flag&wsfsprotocol.SETATTR_MODE != 0 {
		// not support
	}
	if flag&wsfsprotocol.SETATTR_OWNER != 0 {
		// not support
	}
	writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
}
