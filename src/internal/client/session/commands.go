package session

import (
	"bytes"
	"io"
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/rs/zerolog/log"
)

type DirItem struct {
	Name  string
	Size  uint64
	MTime int64
	Mode  uint32
	Owner uint8
	Child []DirItem
	Data  []byte
}

const maxWritePayload int = maxFrameSize - 6 // header(2) + FD(4)

func readDirents(data []byte) ([]wsfsprotocol.Dirent, error) {
	r := bytes.NewReader(data)
	var items []wsfsprotocol.Dirent
	for r.Len() > 0 {
		var ent wsfsprotocol.Dirent
		if err := wsfsprotocol.ReadDirentFromReader(&ent, r); err != nil {
			return nil, err
		}
		items = append(items, ent)
	}
	return items, nil
}

func readRspErrorDesc(data []byte) string {
	var rsp wsfsprotocol.RspError
	if err := wsfsprotocol.ReadRspErrorFromReader(&rsp, bytes.NewReader(data)); err != nil {
		log.Error().Err(err).Msg("Failed to decode error response")
		return ""
	}
	return rsp.Desc
}

func readTreeChunk(data []byte, stack *[]*[]DirItem) (ok bool) {
	r := bytes.NewReader(data)
	current := (*stack)[len(*stack)-1]

	for r.Len() > 0 {
		indicator, err := r.ReadByte()
		if err != nil {
			return false
		}

		switch indicator {
		case wsfsprotocol.TREEDIR_INDICATOR_ENTER_DIR:
			(*current)[len(*current)-1].Child = []DirItem{}
			*stack = append(*stack, &(*current)[len(*current)-1].Child)
			current = (*stack)[len(*stack)-1]
		case wsfsprotocol.TREEDIR_INDICATOR_END_DIR:
			*stack = (*stack)[:len(*stack)-1]
			current = (*stack)[len(*stack)-1]
		case wsfsprotocol.TREEDIR_INDICATOR_END_DIR_WITH_FAIL:
			*stack = (*stack)[:len(*stack)-1]
			*current = nil
			current = (*stack)[len(*stack)-1]
		case wsfsprotocol.TREEDIR_INDICATOR_FILE, wsfsprotocol.TREEDIR_INDICATOR_FILE_WITH_DATA:
			var ent wsfsprotocol.Dirent
			if err := wsfsprotocol.ReadDirentFromReader(&ent, r); err != nil {
				return false
			}
			item := DirItem{
				Name:  ent.Name,
				Size:  ent.Size,
				MTime: ent.MTime,
				Mode:  ent.Mode,
				Owner: ent.Owner,
			}
			if indicator == wsfsprotocol.TREEDIR_INDICATOR_FILE_WITH_DATA {
				item.Data = make([]byte, item.Size)
				if _, err := io.ReadFull(r, item.Data); err != nil {
					return false
				}
			}
			*current = append(*current, item)
		default:
			log.Error().Uint8("Indicator", indicator).Msg("Unknown tree indicator")
			return false
		}
	}
	return true
}

func (s *Session) CmdOpen(path string, oflag uint32, fmode uint32) (uint32, uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdOpen) {
		s.marks[clientMark].Unlock()
		return 0, wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdOpenStructToWriter(wsfsprotocol.CmdOpenStruct{Path: path, OFlag: oflag, FMode: fmode}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return 0, wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code := rspBuf.Bytes[1]

	if code != wsfsprotocol.ErrorOK {
		bufPool.Put(rspBuf)
		return 0, code
	}

	var rsp wsfsprotocol.RspOpen
	err = wsfsprotocol.ReadRspOpenFromReader(&rsp, bytes.NewReader(rspBuf.Bytes[2:rspBuf.Writted()]))
	bufPool.Put(rspBuf)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode CmdOpen response")
		return 0, wsfsprotocol.ErrorIO
	}
	return rsp.FD, code
}

func (s *Session) CmdClose(fd uint32) uint8 {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdClose) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdCloseStructToWriter(wsfsprotocol.CmdCloseStruct{FD: fd}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code := rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return code
}

func (s *Session) CmdRead(fd uint32, dest []byte) (uint64, uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdRead) {
		s.marks[clientMark].Unlock()
		return 0, wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdReadStructToWriter(wsfsprotocol.CmdReadStruct{FD: fd, Size: uint64(len(dest))}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return 0, wsfsprotocol.ErrorIO
	}

	var off int
	for {
		rsp := <-s.responses[clientMark]
		code := rsp.Bytes[1]
		data := rsp.Bytes[2:rsp.Writted()]
		copy(dest[off:], data)
		bufPool.Put(rsp)

		if code == wsfsprotocol.ErrorPartialResponse {
			off += len(data)
			continue
		}
		s.marks[clientMark].Unlock()
		return uint64(off + len(data)), code
	}
}

func (s *Session) CmdReadDir(path string) (list []DirItem, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdReadDir) {
		s.marks[clientMark].Unlock()
		return nil, wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdReadDirStructToWriter(wsfsprotocol.CmdReadDirStruct{Path: path}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return nil, wsfsprotocol.ErrorIO
	}

	for {
		rsp := <-s.responses[clientMark]
		code = rsp.Bytes[1]

		if code != wsfsprotocol.ErrorOK &&
			code != wsfsprotocol.ErrorPartialResponse {
			bufPool.Put(rsp)
			s.marks[clientMark].Unlock()
			return
		}

		dirents, readErr := readDirents(rsp.Bytes[2:rsp.Writted()])
		bufPool.Put(rsp)
		if readErr != nil {
			log.Error().Err(readErr).Msg("Failed to read directory entries")
			s.marks[clientMark].Unlock()
			return nil, wsfsprotocol.ErrorIO
		}

		for _, d := range dirents {
			list = append(list, DirItem{
				Name:  d.Name,
				Size:  d.Size,
				MTime: d.MTime,
				Mode:  d.Mode,
				Owner: d.Owner,
			})
		}

		if code == wsfsprotocol.ErrorPartialResponse {
			continue
		}
		break
	}

	s.marks[clientMark].Unlock()
	return
}

func (s *Session) CmdReadLink(lpath string) (tpath string, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdReadLink) {
		s.marks[clientMark].Unlock()
		return "", wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdReadLinkStructToWriter(wsfsprotocol.CmdReadLinkStruct{Path: lpath}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return "", wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]

	if code != wsfsprotocol.ErrorOK {
		bufPool.Put(rspBuf)
		return
	}

	var rsp wsfsprotocol.RspReadLink
	err = wsfsprotocol.ReadRspReadLinkFromReader(&rsp, bytes.NewReader(rspBuf.Bytes[2:rspBuf.Writted()]))
	bufPool.Put(rspBuf)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode CmdReadLink response")
		return "", wsfsprotocol.ErrorIO
	}
	return rsp.TargetPath, code
}

func (s *Session) CmdWrite(fd uint32, data []byte) (written uint64, code uint8) {
	clientMark := s.newClientMark()

	if len(data) <= maxWritePayload {
		if !s.beginRequest(clientMark, wsfsprotocol.CmdWrite) {
			s.marks[clientMark].Unlock()
			return 0, wsfsprotocol.ErrorIO
		}
		err := wsfsprotocol.WriteCmdWriteStructToWriter(wsfsprotocol.CmdWriteStruct{FD: fd, Data: data}, s.writer)
		s.writeDone(err)
		if err != nil {
			s.marks[clientMark].Unlock()
			return 0, wsfsprotocol.ErrorIO
		}

		rspBuf := <-s.responses[clientMark]
		s.marks[clientMark].Unlock()
		code = rspBuf.Bytes[1]

		if code != wsfsprotocol.ErrorOK {
			bufPool.Put(rspBuf)
			return
		}

		var rsp wsfsprotocol.RspWrite
		err = wsfsprotocol.ReadRspWriteFromReader(&rsp, bytes.NewReader(rspBuf.Bytes[2:rspBuf.Writted()]))
		bufPool.Put(rspBuf)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode CmdWrite response")
			return 0, wsfsprotocol.ErrorIO
		}
		return rsp.Written, code
	}

	var off int
	s.marks[clientMark].Unlock()

	nChunks := len(data) / maxWritePayload
	lastSize := len(data) % maxWritePayload

	for i := 0; i < nChunks; i++ {
		chunk := data[off : off+maxWritePayload]
		n, code := s.CmdWrite(fd, chunk)
		if code != wsfsprotocol.ErrorOK {
			return uint64(off), code
		}
		off += int(n)
	}

	if lastSize > 0 {
		n, code := s.CmdWrite(fd, data[off:])
		if code != wsfsprotocol.ErrorOK {
			return uint64(off), code
		}
		off += int(n)
	}

	return uint64(off), wsfsprotocol.ErrorOK
}

func (s *Session) CmdSeek(fd uint32, flag uint32, off int64) (pos uint64, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdSeek) {
		s.marks[clientMark].Unlock()
		return
	}
	err := wsfsprotocol.WriteCmdSeekStructToWriter(wsfsprotocol.CmdSeekStruct{FD: fd, Flag: flag, Offset: off}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]

	if code != wsfsprotocol.ErrorOK {
		bufPool.Put(rspBuf)
		return
	}

	var rsp wsfsprotocol.RspSeek
	err = wsfsprotocol.ReadRspSeekFromReader(&rsp, bytes.NewReader(rspBuf.Bytes[2:rspBuf.Writted()]))
	bufPool.Put(rspBuf)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode CmdSeek response")
		return 0, wsfsprotocol.ErrorIO
	}
	return rsp.Offset, code
}

func (s *Session) CmdAllocate(fd uint32, flag uint32, off uint64, size uint64) uint8 {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdAllocate) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdAllocateStructToWriter(wsfsprotocol.CmdAllocateStruct{FD: fd, Flag: flag, Offset: off, Size: size}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code := rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return code
}

func (s *Session) CmdGetAttr(fpath string) (fi wsfsprotocol.FileInfo, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdGetAttr) {
		s.marks[clientMark].Unlock()
		return
	}
	err := wsfsprotocol.WriteCmdGetAttrStructToWriter(wsfsprotocol.CmdGetAttrStruct{Path: fpath}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]

	if code != wsfsprotocol.ErrorOK {
		bufPool.Put(rspBuf)
		return
	}

	var rsp wsfsprotocol.RspGetAttr
	err = wsfsprotocol.ReadRspGetAttrFromReader(&rsp, bytes.NewReader(rspBuf.Bytes[2:rspBuf.Writted()]))
	bufPool.Put(rspBuf)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode CmdGetAttr response")
		return fi, wsfsprotocol.ErrorIO
	}
	return rsp.FI, code
}

func (s *Session) CmdSetAttr(fpath string, flag uint8, fi wsfsprotocol.FileInfo) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdSetAttr) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdSetAttrStructToWriter(wsfsprotocol.CmdSetAttrStruct{Path: fpath, Flag: flag, FI: fi}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return
}

func (s *Session) CmdSync(fd uint32) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdSync) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdSyncStructToWriter(wsfsprotocol.CmdSyncStruct{FD: fd}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return
}

func (s *Session) CmdMkdir(fpath string, mode uint32) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdMkdir) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdMkdirStructToWriter(wsfsprotocol.CmdMkdirStruct{Path: fpath, Mode: mode}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return
}

func (s *Session) CmdSymLink(target string, fpath string) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdSymLink) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdSymLinkStructToWriter(wsfsprotocol.CmdSymLinkStruct{TargetPath: target, FilePath: fpath}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return
}

func (s *Session) CmdRemove(fpath string) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdRemove) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdRemoveStructToWriter(wsfsprotocol.CmdRemoveStruct{Path: fpath}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return
}

func (s *Session) CmdRmDir(fpath string) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdRmDir) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdRmDirStructToWriter(wsfsprotocol.CmdRmDirStruct{Path: fpath}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return
}

func (s *Session) CmdFsStat(fpath string) (fsi wsfsprotocol.RspFsStat, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdFsStat) {
		s.marks[clientMark].Unlock()
		return
	}
	err := wsfsprotocol.WriteCmdFsStatStructToWriter(wsfsprotocol.CmdFsStatStruct{Path: fpath}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]

	if code != wsfsprotocol.ErrorOK {
		bufPool.Put(rspBuf)
		return
	}

	err = wsfsprotocol.ReadRspFsStatFromReader(&fsi, bytes.NewReader(rspBuf.Bytes[2:rspBuf.Writted()]))
	bufPool.Put(rspBuf)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode CmdFsStat response")
		return fsi, wsfsprotocol.ErrorIO
	}
	return
}

func (s *Session) CmdReadAt(fd uint32, offset uint64, dest []byte) (uint64, uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdReadAt) {
		s.marks[clientMark].Unlock()
		return 0, wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdReadAtStructToWriter(wsfsprotocol.CmdReadAtStruct{FD: fd, Offset: offset, Size: uint64(len(dest))}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return 0, wsfsprotocol.ErrorIO
	}

	var off int
	for {
		rsp := <-s.responses[clientMark]
		code := rsp.Bytes[1]
		data := rsp.Bytes[2:rsp.Writted()]
		copy(dest[off:], data)
		bufPool.Put(rsp)

		if code == wsfsprotocol.ErrorPartialResponse {
			off += len(data)
			continue
		}
		s.marks[clientMark].Unlock()
		return uint64(off + len(data)), code
	}
}

const maxWriteAtPayload int = maxFrameSize - 14 // header(2) + FD(4) + Offset(8)

func (s *Session) CmdWriteAt(fd uint32, offset uint64, data []byte) (written uint64, code uint8) {
	if len(data) <= maxWriteAtPayload {
		clientMark := s.newClientMark()

		if !s.beginRequest(clientMark, wsfsprotocol.CmdWriteAt) {
			s.marks[clientMark].Unlock()
			return 0, wsfsprotocol.ErrorIO
		}
		err := wsfsprotocol.WriteCmdWriteAtStructToWriter(wsfsprotocol.CmdWriteAtStruct{FD: fd, Offset: offset, Data: data}, s.writer)
		s.writeDone(err)
		if err != nil {
			s.marks[clientMark].Unlock()
			return 0, wsfsprotocol.ErrorIO
		}

		rspBuf := <-s.responses[clientMark]
		s.marks[clientMark].Unlock()
		code = rspBuf.Bytes[1]

		if code != wsfsprotocol.ErrorOK {
			bufPool.Put(rspBuf)
			return
		}

		var rsp wsfsprotocol.RspWriteAt
		err = wsfsprotocol.ReadRspWriteAtFromReader(&rsp, bytes.NewReader(rspBuf.Bytes[2:rspBuf.Writted()]))
		bufPool.Put(rspBuf)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode CmdWriteAt response")
			return 0, wsfsprotocol.ErrorIO
		}
		return rsp.Written, code
	}

	stream, err := s.OpenWriteStream(fd, offset, data)
	if err != nil {
		return 0, wsfsprotocol.ErrorIO
	}
	written, code, _ = stream.Close(nil)
	if code != wsfsprotocol.ErrorOK {
		return written, code
	}

	writeErrCode, _ := stream.WriteError()
	if writeErrCode != 0 {
		return written, writeErrCode
	}

	return written, wsfsprotocol.ErrorOK
}

func (s *Session) CmdRename(old string, new string, mode uint32) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdRename) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdRenameStructToWriter(wsfsprotocol.CmdRenameStruct{OldPath: old, NewPath: new, Flag: mode}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return
}

func (s *Session) CmdCopyFileRange(wfd1 uint32, wfd2 uint32, off1 uint64, off2 uint64, size uint64) (copied uint64, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdCopyFileRange) {
		s.marks[clientMark].Unlock()
		return
	}
	err := wsfsprotocol.WriteCmdCopyFileRangeStructToWriter(wsfsprotocol.CmdCopyFileRangeStruct{
		SrcFD: wfd1, DstFD: wfd2, SrcOffset: off1, DstOffset: off2, Size: size,
	}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]

	if code != wsfsprotocol.ErrorOK {
		bufPool.Put(rspBuf)
		return
	}

	var rsp wsfsprotocol.RspCopyFileRange
	err = wsfsprotocol.ReadRspCopyFileRangeFromReader(&rsp, bytes.NewReader(rspBuf.Bytes[2:rspBuf.Writted()]))
	bufPool.Put(rspBuf)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode CmdCopyFileRange response")
		return 0, wsfsprotocol.ErrorIO
	}
	return rsp.Copied, code
}

func (s *Session) CmdSetAttrByFD(wfd uint32, flag uint8, fi wsfsprotocol.FileInfo) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdSetAttrByFD) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdSetAttrByFDStructToWriter(wsfsprotocol.CmdSetAttrByFDStruct{FD: wfd, Flag: flag, FI: fi}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]
	bufPool.Put(rspBuf)
	return
}

func (s *Session) CmdTreeDir(path string, depth uint8, hint string) (tree []DirItem, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdTreeDir) {
		s.marks[clientMark].Unlock()
		return nil, wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdTreeDirStructToWriter(wsfsprotocol.CmdTreeDirStruct{Path: path, Depth: depth, Hint: hint}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return nil, wsfsprotocol.ErrorIO
	}

	tree = append(tree, DirItem{})
	stack := []*[]DirItem{&tree}

	for {
		rsp := <-s.responses[clientMark]
		code = rsp.Bytes[1]

		if code != wsfsprotocol.ErrorOK &&
			code != wsfsprotocol.ErrorPartialResponse {
			bufPool.Put(rsp)
			s.marks[clientMark].Unlock()
			return
		}

		ok := readTreeChunk(rsp.Bytes[2:rsp.Writted()], &stack)
		bufPool.Put(rsp)
		if !ok {
			log.Error().Msg("Failed to read tree response")
			s.marks[clientMark].Unlock()
			return nil, wsfsprotocol.ErrorIO
		}

		if code == wsfsprotocol.ErrorPartialResponse {
			continue
		}
		break
	}

	if len(tree) != 1 || tree[0].Child == nil {
		code = wsfsprotocol.ErrorIO
	}
	tree = tree[0].Child

	s.marks[clientMark].Unlock()
	return
}
