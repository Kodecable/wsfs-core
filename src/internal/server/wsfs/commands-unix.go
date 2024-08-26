//go:build unix

package wsfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"wsfs-core/internal/server/wsfs/copyfilerange"
	"wsfs-core/internal/server/wsfs/fallocate"
	"wsfs-core/internal/server/wsfs/frsize"
	"wsfs-core/internal/server/wsfs/renameat2"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"
)

type sfd_t int

// copy from src/os/file.go, modifed.
// Copyright 2009 The Go Authors. All rights reserved.
// syscallMode returns the syscall-specific mode bits from Go's portable mode bits.
func syscallMode(i os.FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&os.ModeSetuid != 0 {
		o |= syscall.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= syscall.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= syscall.S_ISVTX
	}
	return
}

// copy from src/os/file_posix.go.
// Copyright 2009 The Go Authors. All rights reserved.
// ignoringEINTR makes a function call and repeats it if it returns an
// EINTR error. This appears to be required even though we install all
// signal handlers with SA_RESTART: see #22838, #38033, #38836, #40846.
// Also #20400 and #36644 are issues in which a signal handler is
// installed without setting SA_RESTART. None of these are the common case,
// but there are enough of them that it seems that we can't avoid
// an EINTR loop.
func ignoringEINTR(fn func() error) error {
	for {
		err := fn()
		if err != syscall.EINTR {
			return err
		}
	}
}

var errorCodeMap map[syscall.Errno]uint8 = map[syscall.Errno]uint8{
	syscall.EACCES: wsfsprotocol.ErrorAccess,
	syscall.EROFS:  wsfsprotocol.ErrorAccess,
	syscall.EFAULT: wsfsprotocol.ErrorAccess,
	syscall.EPERM:  wsfsprotocol.ErrorAccess,
	//syscall.ENOMEM:       CmdErrBusy,
	syscall.EBUSY:        wsfsprotocol.ErrorBusy,
	syscall.EEXIST:       wsfsprotocol.ErrorExists,
	syscall.ENAMETOOLONG: wsfsprotocol.ErrorTooLoong,
	syscall.EINVAL:       wsfsprotocol.ErrorInvail,
	syscall.EBADF:        wsfsprotocol.ErrorInvailFD,
	syscall.ENOENT:       wsfsprotocol.ErrorNotExists,
	syscall.ELOOP:        wsfsprotocol.ErrorLoop,
	syscall.EDQUOT:       wsfsprotocol.ErrorNoSpace,
	syscall.ENOSPC:       wsfsprotocol.ErrorNoSpace,
	syscall.ENOTEMPTY:    wsfsprotocol.ErrorNotEmpty,
	syscall.ENOTDIR:      wsfsprotocol.ErrorType,
	syscall.EIO:          wsfsprotocol.ErrorIO,
	syscall.ENOTSUP:      wsfsprotocol.ErrorNotSupport,
	//syscall.EMLINK:       wsfsprotocol.ErrorLinkMax,
}

func wsfsErrCode(err error) uint8 {
	var syserr syscall.Errno

	if errors.As(err, &syserr) {
		code, ok := errorCodeMap[syserr]
		if ok {
			return code
		} else {
			return wsfsprotocol.ErrorUnknown
		}
	} else {
		return wsfsprotocol.ErrorUnknown
	}
}

func (s *session) convOwner(fi os.FileInfo) (ownerInfo uint8) {
	stat := fi.Sys().(*syscall.Stat_t)
	if stat.Uid == s.handler.suser.Uid {
		ownerInfo += 1
	}
	if stat.Gid == s.handler.suser.Gid {
		ownerInfo += 2
	}
	return
}

func (s *session) cmdOpen(clientMark uint8, writeCh chan<- *util.Buffer, path string, oflag uint32, fmode uint32) {
	defer s.wg.Done()

	if !util.IsUrlValid(path) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + path

	var sfd int
	var err error
	ignoringEINTR(func() error {
		sfd, err = syscall.Open(apath, int(oflag), syscallMode(fs.FileMode(fmode)))
		return err
	})

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
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
	err := syscall.Close(int(sfd))

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func readAndSend(clientMark uint8, writeCh chan<- *util.Buffer, fd int, size uint64, okCode uint8) bool {
	buf := bufPool.Get().(*util.Buffer)
	buf.Put(clientMark)
	buf.Put(okCode)
	readed, err := syscall.Read(fd, buf.DirectPutStart(int(size)))
	buf.DirectPutDone(readed)

	if err != nil {
		buf.Done()
		buf.Put(clientMark)
		buf.Put(wsfsErrCode(err))
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
		readAndSend(clientMark, writeCh, int(sfd), size, wsfsprotocol.ErrorOK)
	} else {
		for range size / dataPerMsg {
			ok := readAndSend(clientMark, writeCh, int(sfd), dataPerMsg, wsfsprotocol.ErrorPartialResponse)
			if !ok {
				return
			}
		}
		if size%dataPerMsg == 0 {
			writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
		} else {
			readAndSend(clientMark, writeCh, int(sfd), size%dataPerMsg, wsfsprotocol.ErrorOK)
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

	var buf [256]byte
	var size int
	var err error
	ignoringEINTR(func() error {
		size, err = syscall.Readlink(lpath, buf[:])
		return err
	})

	if err != nil {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorType, "syscall error")
		return
	}
	target := string(buf[:size])

	if target[0] != '/' {
		target = filepath.Clean(filepath.Join(apath, target))
	}

	if strings.HasPrefix(target, s.storage.Path) {
		target = strings.TrimPrefix(target, s.storage.Path)
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

	// syscall lseek will not return EINTR
	offset, err := syscall.Seek(int(sfd), off, int(flag))

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
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

	var count int
	var err error
	ignoringEINTR(func() error {
		count, err = syscall.Write(int(sfd), data.Done())
		return err
	})
	bufPool.Put(data)

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK, uint64(count))
	}
}

func (s *session) cmdAllocate(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32, flag uint32, off uint64, size uint64) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	err := fallocate.Fallocate(int(sfd), flag, int64(off), int64(size))

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func (s *session) cmdSetAttr(clientMark uint8, writeCh chan<- *util.Buffer, lpath string, flag uint8, size uint64, mtime int64, mode uint32, owner uint8) {
	defer s.wg.Done()

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	if flag&wsfsprotocol.SETATTR_SIZE != 0 {
		err := syscall.Truncate(apath, int64(size))
		if err != nil {
			writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	if flag&wsfsprotocol.SETATTR_MTIME != 0 {
		err := syscall.Utimes(apath, []syscall.Timeval{
			{Sec: timeval(mtime), Usec: 0}, // mtime is int32 in 32 bit arch
			{Sec: timeval(mtime), Usec: 0},
		})
		if err != nil {
			writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	if flag&wsfsprotocol.SETATTR_MODE != 0 {
		err := syscall.Chmod(apath, syscallMode(fs.FileMode(mode)))
		if err != nil {
			writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	if flag&wsfsprotocol.SETATTR_OWNER != 0 {
		uid := s.handler.suser.OtherUid
		gid := s.handler.suser.OtherGid
		if owner&wsfsprotocol.OWNER_UN != 0 {
			uid = s.handler.suser.Uid
		}
		if owner&wsfsprotocol.OWNER_NG != 0 {
			gid = s.handler.suser.Gid
		}
		err := syscall.Chown(apath, int(uid), int(gid))
		if err != nil {
			writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
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

	err := syscall.Fsync(int(sfd))

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
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

	err := syscall.Mkdir(apath, syscallMode(fs.FileMode(mode)))

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
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

	err := syscall.Symlink(aOldPath, aNewPath)

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
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

	err := syscall.Unlink(apath)

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
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

	err := syscall.Rmdir(apath)

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func (s *session) cmdFsStat(clientMark uint8, writeCh chan<- *util.Buffer, lpath string) {
	defer s.wg.Done()
	var stat syscall.Statfs_t

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	err := syscall.Statfs(apath, &stat)

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK,
			uint64(stat.Blocks)*uint64(frsize.Frsize(&stat)),
			uint64(stat.Bfree)*uint64(frsize.Frsize(&stat)),
			uint64(stat.Bavail)*uint64(frsize.Frsize(&stat)),
		)
	}
}

func readAtAndSend(clientMark uint8, writeCh chan<- *util.Buffer, fd int, off uint64, size uint64, okCode uint8) (uint64, bool) {
	buf := bufPool.Get().(*util.Buffer)
	buf.Put(clientMark)
	buf.Put(okCode)
	readed, err := syscall.Pread(fd, buf.DirectPutStart(int(size)), int64(off))
	buf.DirectPutDone(readed)

	if err != nil {
		buf.Done()
		buf.Put(clientMark)
		buf.Put(wsfsErrCode(err))
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
		readAtAndSend(clientMark, writeCh, int(sfd), off, size, wsfsprotocol.ErrorOK)
	} else {
		for range size / dataPerMsg {
			readed, ok := readAtAndSend(clientMark, writeCh, int(sfd), off, dataPerMsg, wsfsprotocol.ErrorPartialResponse)
			if !ok {
				return
			}
			off += readed
		}
		if size%dataPerMsg == 0 {
			writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
		} else {
			readAtAndSend(clientMark, writeCh, int(sfd), off, size%dataPerMsg, wsfsprotocol.ErrorOK)
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

	var count int
	var err error
	ignoringEINTR(func() error {
		count, err = syscall.Pwrite(int(sfd), data.Done(), int64(off))
		return err
	})
	bufPool.Put(data)

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK, uint64(count))
	}
}

func (s *session) cmdCopyFileRange(clientMark uint8, writeCh chan<- *util.Buffer, wfd1 uint32, wfd2 uint32, off1 uint64, off2 uint64, size uint64) {
	defer s.wg.Done()

	rsfd1, ok := s.fds.Load(wfd1)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd1 := rsfd1.(sfd_t)

	rsfd2, ok := s.fds.Load(wfd2)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd2 := rsfd2.(sfd_t)

	off1i := int64(off1)
	off2i := int64(off2)
	writed, err := copyfilerange.CopyFileRange(int(sfd1), &off1i, int(sfd2), &off2i, int(size), 0)

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK, uint64(writed))
	}
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
		err = syscall.Rename(aOldPath, aNewPath)
	} else {
		var fd1, fd2 int
		fd1, err = syscall.Open(filepath.Dir(aOldPath), syscall.O_DIRECTORY, 0)
		if err == nil {
			fd2, err = syscall.Open(filepath.Dir(aNewPath), syscall.O_DIRECTORY, 0)
			if err == nil {
				err = renameat2.Renameat2(fd1, aOldPath, fd2, aNewPath, flag)
				syscall.Close(fd2)
			}
			syscall.Close(fd1)
		}
	}

	if err != nil {
		writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
	}
}

func (s *session) cmdSetAttrByFD(clientMark uint8, writeCh chan<- *util.Buffer, wfd uint32, flag uint8, size uint64, mtime int64, mode uint32, owner uint8) {
	defer s.wg.Done()

	rsfd, ok := s.fds.Load(wfd)
	if !ok {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	if flag&wsfsprotocol.SETATTR_SIZE != 0 {
		err := syscall.Ftruncate(int(sfd), int64(size))
		if err != nil {
			writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
		}
	}
	if flag&wsfsprotocol.SETATTR_MTIME != 0 {
		err := syscall.Futimes(int(sfd), []syscall.Timeval{
			{Sec: timeval(mtime), Usec: 0},
			{Sec: timeval(mtime), Usec: 0},
		})
		if err != nil {
			writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
		}
	}
	if flag&wsfsprotocol.SETATTR_MODE != 0 {
		err := syscall.Fchmod(int(sfd), syscallMode(fs.FileMode(mode)))
		if err != nil {
			writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
		}
	}
	if flag&wsfsprotocol.SETATTR_OWNER != 0 {
		uid := s.handler.suser.OtherUid
		gid := s.handler.suser.OtherGid
		if owner&wsfsprotocol.OWNER_UN != 0 {
			uid = s.handler.suser.Uid
		}
		if owner&wsfsprotocol.OWNER_NG != 0 {
			gid = s.handler.suser.Gid
		}
		err := syscall.Fchown(int(sfd), int(uid), int(gid))
		if err != nil {
			writeCh <- msg(clientMark, wsfsErrCode(err), "syscall error")
		}
	}
	writeCh <- msg(clientMark, wsfsprotocol.ErrorOK)
}
