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
	"wsfs-core/internal/server/wsfs/ofdlock"
	"wsfs-core/internal/server/wsfs/reflink"
	"wsfs-core/internal/server/wsfs/renameat2"
	"wsfs-core/internal/server/wsfs/timeval"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/share/wsfsunixconv"
	"wsfs-core/internal/util"
)

type sfd_t int

func closeSFD(fd sfd_t) {
	_ = syscall.Close(int(fd))
}

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
	syscall.EACCES: wsfsprotocol.ErrorAccessRestricted,
	syscall.EROFS:  wsfsprotocol.ErrorAccessRestricted,
	syscall.EPERM:  wsfsprotocol.ErrorStateBlocked,
	//syscall.ENOMEM:       CmdErrBusy,
	syscall.EBUSY:        wsfsprotocol.ErrorBusy,
	syscall.EEXIST:       wsfsprotocol.ErrorExists,
	syscall.EXDEV:        wsfsprotocol.ErrorCrossDevice,
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
	syscall.ETXTBSY:      wsfsprotocol.ErrorSpecialFileBlocked,
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
	if stat.Uid == s.handler.fsIds.Uid {
		ownerInfo += 1
	}
	if stat.Gid == s.handler.fsIds.Gid {
		ownerInfo += 2
	}
	return
}

func (s *session) cmdOpen(clientMark uint8, req wsfsprotocol.CmdOpenStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + req.Path

	oflag := 0
	switch req.OFlag & wsfsprotocol.O_ACCMODE {
	case wsfsprotocol.O_RDONLY:
		oflag |= wsfsunixconv.OpenFlagToUnix[wsfsprotocol.O_RDONLY]
	case wsfsprotocol.O_WRONLY:
		oflag |= wsfsunixconv.OpenFlagToUnix[wsfsprotocol.O_WRONLY]
	case wsfsprotocol.O_RDWR:
		oflag |= wsfsunixconv.OpenFlagToUnix[wsfsprotocol.O_RDWR]
	default:
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad open access mode")
		return
	}
	for _, proctocolFlag := range wsfsprotocol.OpenFlags {
		if proctocolFlag&wsfsprotocol.O_ACCMODE != 0 {
			continue
		}
		if req.OFlag&proctocolFlag != 0 {
			unixFlag, ok := wsfsunixconv.OpenFlagToUnix[proctocolFlag]
			if !ok {
				s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "not supported open flag")
				return
			}
			oflag |= unixFlag
		}
	}

	var sfd int
	var err error
	ignoringEINTR(func() error {
		sfd, err = syscall.Open(apath, oflag, syscallMode(fs.FileMode(req.FMode)))
		return err
	})

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		wfd := s.newFD(sfd_t(sfd))
		if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
			err = wsfsprotocol.WriteRspOpenToWriter(wsfsprotocol.RspOpen{FD: wfd}, s.writer)
			s.writeDone(err)
		}
	}
}

func (s *session) cmdClose(clientMark uint8, req wsfsprotocol.CmdCloseStruct) {

	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	// TODO: Carefully handle EINTR
	// when close() return EINTR. Linux and AIX typically close the file
	// descriptor despite interruption, whereas HPUX may keep the descriptor
	// open.
	s.fds.Delete(req.FD)
	err := syscall.Close(int(sfd))

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		s.writeRspOK(clientMark)
	}
}

func (s *session) readAndSend(clientMark uint8, fd int, size uint64, okCode uint8) bool {
	buf := bufPool.Get().(*util.Buffer)
	defer putBuf(buf)
	buf.Write([]byte{clientMark, okCode})
	readed, err := syscall.Read(fd, buf.Bytes[buf.Writted():][:int(size)])
	buf.Grow(readed)

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
		return false
	}
	s.write(buf.Done())
	return true
}

func (s *session) cmdRead(clientMark uint8, req wsfsprotocol.CmdReadStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	if req.Size < maxReadPayLoad {
		s.readAndSend(clientMark, int(sfd), req.Size, wsfsprotocol.ErrorOK)
	} else {
		for range req.Size / maxReadPayLoad {
			ok := s.readAndSend(clientMark, int(sfd), maxReadPayLoad, wsfsprotocol.ErrorPartialResponse)
			if !ok {
				return
			}
		}
		if req.Size%maxReadPayLoad == 0 {
			s.writeRspOK(clientMark)
		} else {
			s.readAndSend(clientMark, int(sfd), req.Size%maxReadPayLoad, wsfsprotocol.ErrorOK)
		}
	}
}

func (s *session) cmdReadLink(clientMark uint8, req wsfsprotocol.CmdReadLinkStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + req.Path

	var buf [256]byte
	var size int
	var err error
	ignoringEINTR(func() error {
		size, err = syscall.Readlink(apath, buf[:])
		return err
	})

	if err != nil {
		s.writeRspError(clientMark, wsfsprotocol.ErrorType, "syscall error")
		return
	}
	target := string(buf[:size])

	if target[0] != '/' {
		target = filepath.Clean(filepath.Join(apath, target))
	}

	if strings.HasPrefix(target, s.storage.Path) {
		target = strings.TrimPrefix(target, s.storage.Path)
		if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
			err = wsfsprotocol.WriteRspReadLinkToWriter(wsfsprotocol.RspReadLink{TargetPath: target}, s.writer)
			s.writeDone(err)
		}
	} else {
		// we will handle this kind symlinks in getattr and readdir to
		// prevent escape storage root. So, fake it.
		s.writeRspError(clientMark, wsfsprotocol.ErrorType, "syscall error")
	}
}

func (s *session) cmdSeek(clientMark uint8, req wsfsprotocol.CmdSeekStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	whence, ok := wsfsunixconv.WhenceToUnix[req.Whence]
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "whence not supported")
		return
	}

	// syscall lseek will not return EINTR
	offset, err := syscall.Seek(int(sfd), req.Offset, whence)

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
			err = wsfsprotocol.WriteRspSeekToWriter(wsfsprotocol.RspSeek{Offset: uint64(offset)}, s.writer)
			s.writeDone(err)
		}
	}
}

func (s *session) cmdWrite(clientMark uint8, req wsfsprotocol.CmdWriteStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	var count int
	var err error
	ignoringEINTR(func() error {
		count, err = syscall.Write(int(sfd), req.Data)
		return err
	})

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
			err = wsfsprotocol.WriteRspWriteToWriter(wsfsprotocol.RspWrite{Written: uint64(count)}, s.writer)
			s.writeDone(err)
		}
	}
}

func (s *session) cmdAllocate(clientMark uint8, req wsfsprotocol.CmdAllocateStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	err := fallocate.Fallocate(int(sfd), req.Flag, int64(req.Offset), int64(req.Size))

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		s.writeRspOK(clientMark)
	}
}

func (s *session) cmdSetAttr(clientMark uint8, req wsfsprotocol.CmdSetAttrStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + req.Path

	if req.Flag&wsfsprotocol.SETATTR_SIZE != 0 {
		err := syscall.Truncate(apath, int64(req.FI.Size))
		if err != nil {
			s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	if req.Flag&wsfsprotocol.SETATTR_MTIME != 0 {
		err := timeval.SetPathMTime(apath, req.FI.MTime)
		if err != nil {
			s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	if req.Flag&wsfsprotocol.SETATTR_MODE != 0 {
		err := syscall.Chmod(apath, syscallMode(fs.FileMode(req.FI.Mode)))
		if err != nil {
			s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	if req.Flag&wsfsprotocol.SETATTR_OWNER != 0 {
		uid := s.handler.fsIds.OtherUid
		gid := s.handler.fsIds.OtherGid
		if req.FI.Owner&wsfsprotocol.OWNER_UN != 0 {
			uid = s.handler.fsIds.Uid
		}
		if req.FI.Owner&wsfsprotocol.OWNER_NG != 0 {
			gid = s.handler.fsIds.Gid
		}
		err := syscall.Chown(apath, int(uid), int(gid))
		if err != nil {
			s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdSync(clientMark uint8, req wsfsprotocol.CmdSyncStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	err := syscall.Fsync(int(sfd))

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		s.writeRspOK(clientMark)
	}
}

func (s *session) cmdMkdir(clientMark uint8, req wsfsprotocol.CmdMkdirStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + req.Path

	err := syscall.Mkdir(apath, syscallMode(fs.FileMode(req.Mode)))

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		s.writeRspOK(clientMark)
	}
}

func (s *session) cmdSymLink(clientMark uint8, req wsfsprotocol.CmdSymLinkStruct) {
	if !util.IsUrlValid(req.TargetPath) || !util.IsUrlValid(req.FilePath) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	aOldPath := s.storage.Path + req.TargetPath
	aNewPath := s.storage.Path + req.FilePath

	err := syscall.Symlink(aOldPath, aNewPath)

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		s.writeRspOK(clientMark)
	}
}

func (s *session) cmdRemove(clientMark uint8, req wsfsprotocol.CmdRemoveStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + req.Path

	err := syscall.Unlink(apath)

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		s.writeRspOK(clientMark)
	}
}

func (s *session) cmdRmDir(clientMark uint8, req wsfsprotocol.CmdRmDirStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + req.Path

	err := syscall.Rmdir(apath)

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		s.writeRspOK(clientMark)
	}
}

func (s *session) cmdFsStat(clientMark uint8, req wsfsprotocol.CmdFsStatStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + req.Path

	total, free, avail, err := util.FsSize(apath)

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
			err = wsfsprotocol.WriteRspFsStatToWriter(wsfsprotocol.RspFsStat{Total: total, Free: free, Available: avail}, s.writer)
			s.writeDone(err)
		}
	}
}

func (s *session) readAtAndSend(clientMark uint8, fd int, off uint64, size uint64, okCode uint8) (uint64, bool) {
	buf := bufPool.Get().(*util.Buffer)
	defer putBuf(buf)
	buf.Write([]byte{clientMark, okCode})
	readed, err := syscall.Pread(fd, buf.Bytes[buf.Writted():][:int(size)], int64(off))
	buf.Grow(readed)

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
		return 0, false
	}
	s.write(buf.Done())
	return uint64(readed), true
}

func (s *session) cmdReadAt(clientMark uint8, req wsfsprotocol.CmdReadAtStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)
	off := req.Offset

	if req.Size < maxReadPayLoad {
		s.readAtAndSend(clientMark, int(sfd), off, req.Size, wsfsprotocol.ErrorOK)
	} else {
		for range req.Size / maxReadPayLoad {
			readed, ok := s.readAtAndSend(clientMark, int(sfd), off, maxReadPayLoad, wsfsprotocol.ErrorPartialResponse)
			if !ok {
				return
			}
			off += readed
		}
		if req.Size%maxReadPayLoad == 0 {
			s.writeRspOK(clientMark)
		} else {
			s.readAtAndSend(clientMark, int(sfd), off, req.Size%maxReadPayLoad, wsfsprotocol.ErrorOK)
		}
	}
}

func (s *session) cmdWriteAt(clientMark uint8, req wsfsprotocol.CmdWriteAtStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	var count int
	var err error
	ignoringEINTR(func() error {
		count, err = syscall.Pwrite(int(sfd), req.Data, int64(req.Offset))
		return err
	})

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
			err = wsfsprotocol.WriteRspWriteAtToWriter(wsfsprotocol.RspWriteAt{Written: uint64(count)}, s.writer)
			s.writeDone(err)
		}
	}
}

func (s *session) writeStreamChunk(clientMark uint8, stream *writeStreamState, data []byte) {
	if stream.writeErrSent || len(data) == 0 {
		return
	}

	for len(data) > 0 {
		var written int
		var err error
		ignoringEINTR(func() error {
			written, err = syscall.Pwrite(int(stream.fd), data, int64(stream.offset))
			return err
		})
		if err != nil {
			s.markWriteStreamError(clientMark, stream, wsfsErrCode(err), "syscall error")
			return
		}
		if written == 0 {
			s.markWriteStreamError(clientMark, stream, wsfsprotocol.ErrorIO, "short write")
			return
		}
		stream.offset += uint64(written)
		stream.written += uint64(written)
		data = data[written:]
	}
}

func (s *session) cmdCopyFileRange(clientMark uint8, req wsfsprotocol.CmdCopyFileRangeStruct) {
	rsfd1, ok := s.fds.Load(req.SrcFD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd1 := rsfd1.(sfd_t)

	rsfd2, ok := s.fds.Load(req.DstFD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd2 := rsfd2.(sfd_t)

	off1i := int64(req.SrcOffset)
	off2i := int64(req.DstOffset)
	writed, err := copyfilerange.CopyFileRange(int(sfd1), &off1i, int(sfd2), &off2i, int(req.Size), 0)

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
			err = wsfsprotocol.WriteRspCopyFileRangeToWriter(wsfsprotocol.RspCopyFileRange{Copied: uint64(writed)}, s.writer)
			s.writeDone(err)
		}
	}
}

func (s *session) cmdCloneFileRange(clientMark uint8, req wsfsprotocol.CmdCloneFileRangeStruct) {
	rsfd1, ok := s.fds.Load(req.SrcFD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd1 := rsfd1.(sfd_t)

	rsfd2, ok := s.fds.Load(req.DstFD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd2 := rsfd2.(sfd_t)

	err := reflink.CloneFileRange(int(sfd2), int(sfd1), req.DstOffset, req.SrcOffset, req.Size)
	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
		return
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdRename(clientMark uint8, req wsfsprotocol.CmdRenameStruct) {
	if !util.IsUrlValid(req.OldPath) || !util.IsUrlValid(req.NewPath) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	aOldPath := s.storage.Path + req.OldPath
	aNewPath := s.storage.Path + req.NewPath

	var err error
	if req.Flag == 0 {
		err = syscall.Rename(aOldPath, aNewPath)
	} else {
		var fd1, fd2 int
		fd1, err = syscall.Open(filepath.Dir(aOldPath), syscall.O_DIRECTORY, 0)
		if err == nil {
			fd2, err = syscall.Open(filepath.Dir(aNewPath), syscall.O_DIRECTORY, 0)
			if err == nil {
				err = renameat2.Renameat2(fd1, aOldPath, fd2, aNewPath, req.Flag)
				syscall.Close(fd2)
			}
			syscall.Close(fd1)
		}
	}

	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
	} else {
		s.writeRspOK(clientMark)
	}
}

func (s *session) cmdSetAttrByFD(clientMark uint8, req wsfsprotocol.CmdSetAttrByFDStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	sfd := rsfd.(sfd_t)

	if req.Flag&wsfsprotocol.SETATTR_SIZE != 0 {
		err := syscall.Ftruncate(int(sfd), int64(req.FI.Size))
		if err != nil {
			s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	if req.Flag&wsfsprotocol.SETATTR_MTIME != 0 {
		err := timeval.SetFDMTime(int(sfd), req.FI.MTime)
		if err != nil {
			s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	if req.Flag&wsfsprotocol.SETATTR_MODE != 0 {
		err := syscall.Fchmod(int(sfd), syscallMode(fs.FileMode(req.FI.Mode)))
		if err != nil {
			s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	if req.Flag&wsfsprotocol.SETATTR_OWNER != 0 {
		uid := s.handler.fsIds.OtherUid
		gid := s.handler.fsIds.OtherGid
		if req.FI.Owner&wsfsprotocol.OWNER_UN != 0 {
			uid = s.handler.fsIds.Uid
		}
		if req.FI.Owner&wsfsprotocol.OWNER_NG != 0 {
			gid = s.handler.fsIds.Gid
		}
		err := syscall.Fchown(int(sfd), int(uid), int(gid))
		if err != nil {
			s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
			return
		}
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdGetFileLock(clientMark uint8, req wsfsprotocol.CmdGetFileLockStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}

	outLock, err := ofdlock.GetLock(int(rsfd.(sfd_t)), req.FileLock)
	if err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
		return
	}

	if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		err = wsfsprotocol.WriteRspGetFileLockToWriter(wsfsprotocol.RspGetFileLock{FileLock: outLock}, s.writer)
		s.writeDone(err)
	}
}

func (s *session) cmdSetFileLock(clientMark uint8, req wsfsprotocol.CmdSetFileLockStruct) {
	s.cmdSetFileLockCommon(clientMark, req.FD, req.FileLock, false)
}

func (s *session) cmdSetFileLockWait(clientMark uint8, req wsfsprotocol.CmdSetFileLockWaitStruct) {
	s.cmdSetFileLockCommon(clientMark, req.FD, req.FileLock, true)
}

func (s *session) cmdSetFileLockCommon(clientMark uint8, fd uint32, lock wsfsprotocol.FileLockInfo, blocking bool) {
	rsfd, ok := s.fds.Load(fd)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvailFD, "bad fd")
		return
	}
	if err := ofdlock.SetLock(int(rsfd.(sfd_t)), lock, blocking); err != nil {
		s.writeRspError(clientMark, wsfsErrCode(err), "syscall error")
		return
	}
	s.writeRspOK(clientMark)
}
