//go:build !unix

package wsfs

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"wsfs-core/internal/server/wsfs/timeval"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/share/wsfsstdconv"
	"wsfs-core/internal/util"
)

type sfd_t *os.File

func closeSFD(fd sfd_t) {
	_ = (*os.File)(fd).Close()
}

func (s *session) convOwner(_ os.FileInfo) (ownerInfo uint8) {
	return wsfsprotocol.OWNER_UG
}

func (s *session) cmdOpen(clientMark uint8, req wsfsprotocol.CmdOpenStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalid, "bad path")
		return
	}

	oflag := 0
	switch req.OFlag & wsfsprotocol.O_ACCMODE {
	case wsfsprotocol.O_RDONLY:
		oflag |= wsfsstdconv.OpenFlagToStd[wsfsprotocol.O_RDONLY]
	case wsfsprotocol.O_WRONLY:
		oflag |= wsfsstdconv.OpenFlagToStd[wsfsprotocol.O_WRONLY]
	case wsfsprotocol.O_RDWR:
		oflag |= wsfsstdconv.OpenFlagToStd[wsfsprotocol.O_RDWR]
	default:
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalid, "bad open access mode")
		return
	}
	for _, proctocolFlag := range wsfsprotocol.OpenFlags {
		if proctocolFlag&wsfsprotocol.O_ACCMODE != 0 {
			continue
		}
		if req.OFlag&proctocolFlag != 0 {
			unixFlag, ok := wsfsstdconv.OpenFlagToStd[proctocolFlag]
			if !ok {
				s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "not supported open flag")
				return
			}
			oflag |= unixFlag
		}
	}

	sfd, err := openSFD(s.storage.Path+req.Path, oflag, fs.FileMode(req.FMode))
	if err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}

	if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		err = wsfsprotocol.WriteRspOpenToWriter(wsfsprotocol.RspOpen{FD: s.newFD(sfd_t(sfd))}, s.writer)
		s.writeDone(err)
	}
}

func (s *session) cmdClose(clientMark uint8, req wsfsprotocol.CmdCloseStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalidFD, "bad fd")
		return
	}
	s.fds.Delete(req.FD)

	if err := (*os.File)(rsfd.(sfd_t)).Close(); err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	s.writeRspOK(clientMark)
}

func (s *session) readAndSend(clientMark uint8, fd *os.File, size uint64, partial bool) (uint64, bool) {
	buf := bufPool.Get().(*util.Buffer)
	defer putBuf(buf)
	buf.Write([]byte{clientMark, wsfsprotocol.ErrorOK})
	readed, err := fd.Read(buf.Bytes[buf.Written():][:int(size)])
	buf.Grow(readed)

	if err != nil && err != io.EOF {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return 0, false
	}
	if partial && uint64(readed) == size {
		buf.Bytes[1] = wsfsprotocol.ErrorPartialResponse
	}
	s.write(buf.Done())
	return uint64(readed), true
}

func (s *session) cmdRead(clientMark uint8, req wsfsprotocol.CmdReadStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalidFD, "bad fd")
		return
	}
	sfd := (*os.File)(rsfd.(sfd_t))

	if req.Size < maxReadPayLoad {
		s.readAndSend(clientMark, sfd, req.Size, false)
		return
	}
	for range req.Size / maxReadPayLoad {
		readed, ok := s.readAndSend(clientMark, sfd, maxReadPayLoad, true)
		if !ok {
			return
		}
		if readed < maxReadPayLoad {
			return
		}
	}
	if req.Size%maxReadPayLoad == 0 {
		s.writeRspOK(clientMark)
	} else {
		s.readAndSend(clientMark, sfd, req.Size%maxReadPayLoad, false)
	}
}

func (s *session) cmdReadLink(clientMark uint8, req wsfsprotocol.CmdReadLinkStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalid, "bad path")
		return
	}
	apath := s.storage.Path + req.Path
	target, err := os.Readlink(apath)
	if err != nil {
		s.writeRspError(clientMark, wsfsprotocol.ErrorType, "syscall error")
		return
	}

	if target[0] != '/' {
		target = filepath.Clean(filepath.Join(apath, target))
	}
	if !strings.HasPrefix(target, s.storage.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorType, "syscall error")
		return
	}

	if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		err = wsfsprotocol.WriteRspReadLinkToWriter(wsfsprotocol.RspReadLink{TargetPath: strings.TrimPrefix(target, s.storage.Path)}, s.writer)
		s.writeDone(err)
	}
}

func (s *session) cmdSeek(clientMark uint8, req wsfsprotocol.CmdSeekStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalidFD, "bad fd")
		return
	}

	whence, ok := wsfsstdconv.WhenceToStd[req.Whence]
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "whence not supported")
		return
	}

	offset, err := (*os.File)(rsfd.(sfd_t)).Seek(req.Offset, whence)
	if err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		err = wsfsprotocol.WriteRspSeekToWriter(wsfsprotocol.RspSeek{Offset: uint64(offset)}, s.writer)
		s.writeDone(err)
	}
}

func (s *session) cmdWrite(clientMark uint8, req wsfsprotocol.CmdWriteStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalidFD, "bad fd")
		return
	}
	count, err := (*os.File)(rsfd.(sfd_t)).Write(req.Data)
	if err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		err = wsfsprotocol.WriteRspWriteToWriter(wsfsprotocol.RspWrite{Written: uint64(count)}, s.writer)
		s.writeDone(err)
	}
}

func (s *session) cmdAllocate(clientMark uint8, _ wsfsprotocol.CmdAllocateStruct) {
	s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
}

func (s *session) cmdSetAttr(clientMark uint8, req wsfsprotocol.CmdSetAttrStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalid, "bad path")
		return
	}
	apath := s.storage.Path + req.Path
	if req.Flag&wsfsprotocol.SETATTR_SIZE != 0 {
		if err := os.Truncate(apath, int64(req.FI.Size)); err != nil {
			s.writeRspError(clientMark, osErrCode(err), "syscall error")
			return
		}
	}
	if req.Flag&wsfsprotocol.SETATTR_MTIME != 0 {
		if err := timeval.SetPathMTime(apath, req.FI.MTime); err != nil {
			s.writeRspError(clientMark, osErrCode(err), "syscall error")
			return
		}
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdSync(clientMark uint8, req wsfsprotocol.CmdSyncStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalidFD, "bad fd")
		return
	}
	if err := (*os.File)(rsfd.(sfd_t)).Sync(); err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdMkdir(clientMark uint8, req wsfsprotocol.CmdMkdirStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalid, "bad path")
		return
	}
	if err := os.Mkdir(s.storage.Path+req.Path, fs.FileMode(req.Mode)); err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdSymLink(clientMark uint8, req wsfsprotocol.CmdSymLinkStruct) {
	if !util.IsUrlValid(req.TargetPath) || !util.IsUrlValid(req.FilePath) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalid, "bad path")
		return
	}
	if err := os.Symlink(s.storage.Path+req.TargetPath, s.storage.Path+req.FilePath); err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdRemove(clientMark uint8, req wsfsprotocol.CmdRemoveStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalid, "bad path")
		return
	}
	resolvedPath := s.storage.Path + req.Path
	if err := os.Remove(resolvedPath); err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdRmDir(clientMark uint8, req wsfsprotocol.CmdRmDirStruct) {
	//log.Debug().Uint8("Cm", clientMark).Str("Path", req.Path).Msg("Removing directory")
	s.cmdRemove(clientMark, wsfsprotocol.CmdRemoveStruct{Path: req.Path})
}

func (s *session) readAtAndSend(clientMark uint8, fd *os.File, off uint64, size uint64, partial bool) (uint64, bool) {
	buf := bufPool.Get().(*util.Buffer)
	defer putBuf(buf)
	buf.Write([]byte{clientMark, wsfsprotocol.ErrorOK})
	readed, err := fd.ReadAt(buf.Bytes[buf.Written():][:int(size)], int64(off))
	buf.Grow(readed)

	if err != nil && err != io.EOF {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return 0, false
	}
	if partial && uint64(readed) == size {
		buf.Bytes[1] = wsfsprotocol.ErrorPartialResponse
	}
	s.write(buf.Done())
	return uint64(readed), true
}

func (s *session) cmdReadAt(clientMark uint8, req wsfsprotocol.CmdReadAtStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalidFD, "bad fd")
		return
	}
	sfd := (*os.File)(rsfd.(sfd_t))
	off := req.Offset

	if req.Size < maxReadPayLoad {
		s.readAtAndSend(clientMark, sfd, off, req.Size, false)
		return
	}
	for range req.Size / maxReadPayLoad {
		readed, ok := s.readAtAndSend(clientMark, sfd, off, maxReadPayLoad, true)
		if !ok {
			return
		}
		if readed < maxReadPayLoad {
			return
		}
		off += readed
	}
	if req.Size%maxReadPayLoad == 0 {
		s.writeRspOK(clientMark)
	} else {
		s.readAtAndSend(clientMark, sfd, off, req.Size%maxReadPayLoad, false)
	}
}

func (s *session) cmdWriteAt(clientMark uint8, req wsfsprotocol.CmdWriteAtStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalidFD, "bad fd")
		return
	}
	count, err := (*os.File)(rsfd.(sfd_t)).WriteAt(req.Data, int64(req.Offset))
	if err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		err = wsfsprotocol.WriteRspWriteAtToWriter(wsfsprotocol.RspWriteAt{Written: uint64(count)}, s.writer)
		s.writeDone(err)
	}
}

func writeStreamWriteChunk(fd sfd_t, offset uint64, data []byte) (uint64, uint8, string, bool) {
	var totalWritten uint64
	for len(data) > 0 {
		written, err := (*os.File)(fd).WriteAt(data, int64(offset))
		if err != nil {
			return totalWritten, osErrCode(err), "syscall error", false
		}
		if written == 0 {
			return totalWritten, wsfsprotocol.ErrorIO, "short write", false
		}
		offset += uint64(written)
		totalWritten += uint64(written)
		data = data[written:]
	}
	return totalWritten, 0, "", true
}

func (s *session) cmdCopyFileRange(clientMark uint8, _ wsfsprotocol.CmdCopyFileRangeStruct) {
	s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
}

func (s *session) cmdCloneFileRange(clientMark uint8, _ wsfsprotocol.CmdCloneFileRangeStruct) {
	s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
}

func (s *session) cmdGetFileLock(clientMark uint8, _ wsfsprotocol.CmdGetFileLockStruct) {
	s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
}

func (s *session) cmdSetFileLock(clientMark uint8, _ wsfsprotocol.CmdSetFileLockStruct) {
	s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
}

func (s *session) cmdSetFileLockWait(clientMark uint8, _ wsfsprotocol.CmdSetFileLockWaitStruct) {
	s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
}

func (s *session) cmdRename(clientMark uint8, req wsfsprotocol.CmdRenameStruct) {
	if !util.IsUrlValid(req.OldPath) || !util.IsUrlValid(req.NewPath) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalid, "bad path")
		return
	}
	if req.Flag != 0 {
		s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
		return
	}
	resolvedOldPath := s.storage.Path + req.OldPath
	resolvedNewPath := s.storage.Path + req.NewPath
	if err := os.Rename(resolvedOldPath, resolvedNewPath); err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdSetAttrByFD(clientMark uint8, req wsfsprotocol.CmdSetAttrByFDStruct) {
	rsfd, ok := s.fds.Load(req.FD)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalidFD, "bad fd")
		return
	}
	if req.Flag&wsfsprotocol.SETATTR_SIZE != 0 {
		if err := (*os.File)(rsfd.(sfd_t)).Truncate(int64(req.FI.Size)); err != nil {
			s.writeRspError(clientMark, osErrCode(err), "syscall error")
			return
		}
	}
	s.writeRspOK(clientMark)
}

func (s *session) cmdFsStat(clientMark uint8, req wsfsprotocol.CmdFsStatStruct) {
	if !util.IsUrlValid(req.Path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvalid, "bad path")
		return
	}
	total, free, avail, err := util.FsSize(s.storage.Path + req.Path)
	if err != nil {
		s.writeRspError(clientMark, osErrCode(err), "syscall error")
		return
	}
	if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		err = wsfsprotocol.WriteRspFsStatToWriter(wsfsprotocol.RspFsStat{Total: total, Free: free, Available: avail}, s.writer)
		s.writeDone(err)
	}
}

func (s *session) cmdLink(clientMark uint8, _ wsfsprotocol.CmdLinkStruct) {
	s.writeRspError(clientMark, wsfsprotocol.ErrorNotSupport, "syscall error")
}
