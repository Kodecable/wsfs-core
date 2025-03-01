package session

import (
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog/log"
)

func (s *Session) CmdOpen(path string, oflag uint32, fmode uint32) (uint32, uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdOpen, path, oflag, fmode))

	code := buf.ReadByteAt(1)
	if code != wsfsprotocol.ErrorOK {
		buf.Done()
		bufPool.Put(buf)
		return 0, code
	}

	if !buf.Ensure(5) {
		buf.Done()
		bufPool.Put(buf)
		log.Error().Msg("Command response too short")
		return 0, wsfsprotocol.ErrorIO
	}

	fd := buf.ReadU32(2)
	buf.Done()
	bufPool.Put(buf)
	return fd, code
}

func (s *Session) CmdClose(fd uint32) uint8 {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdClose, fd))
	code := buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return code
}

func (s *Session) CmdRead(fd uint32, dest []byte) (uint64, uint8) {
	clientMark := s.newClientMark()
	s.writeRequest <- msg(clientMark, wsfsprotocol.CmdRead, fd, uint64(len(dest)))

	var off int
	for {
		rsp := <-s.readRequests[clientMark]

		code := rsp.ReadByteAt(1)
		data := rsp.Done()[2:]
		copy(dest[off:], data)
		bufPool.Put(rsp)

		if code == wsfsprotocol.ErrorPartialResponse {
			off += len(data)
			continue
		} else {
			s.clientMarks[clientMark].Unlock()
			return uint64(off + len(data)), code
		}
	}
}

// A vaild DirItem has a non-empty Name
type DirItem struct {
	Name  string
	Size  uint64
	MTime int64
	Mode  uint32
	Owner uint8
	Child []DirItem
	Data  []byte
}

func readList(data *util.Buffer, items *[]DirItem) (ok bool) {
	ok = true
	off := 2

	var di DirItem
	for {
		if !data.Ensure(off) {
			break
		}

		di.Name, ok = data.ReadString(off)
		if !ok {
			return
		}
		off += len(di.Name) + 1

		ok = data.Ensure(off + 20)
		if !ok {
			return
		}
		di.Size = data.ReadU64(off)
		di.MTime = int64(data.ReadU64(off + 8))
		di.Mode = data.ReadU32(off + 8 + 8)
		di.Owner = data.ReadByteAt(off + 8 + 8 + 4)
		//di.Child = nil
		//di.Data = nil
		off += 21 // more 1 byte for loop

		*items = append(*items, di)
	}

	return
}

func (s *Session) CmdReadDir(path string) (list []DirItem, code uint8) {
	clientMark := s.newClientMark()
	s.writeRequest <- msg(clientMark, wsfsprotocol.CmdReadDir, path)

	var rsp *util.Buffer
	for {
		rsp = <-s.readRequests[clientMark]

		code = rsp.ReadByteAt(1)
		if code != wsfsprotocol.ErrorOK &&
			code != wsfsprotocol.ErrorPartialResponse {
			rsp.Done()
			bufPool.Put(rsp)
			return
		}

		ok := readList(rsp, &list)
		rsp.Done()
		bufPool.Put(rsp)
		if !ok {
			log.Error().Msg("Command response too short")
			s.clientMarks[clientMark].Unlock()
			return nil, wsfsprotocol.ErrorIO
		}

		if code == wsfsprotocol.ErrorPartialResponse {
			continue
		} else {
			break
		}
	}

	s.clientMarks[clientMark].Unlock()
	return
}

func (s *Session) CmdReadLink(lpath string) (tpath string, code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdReadLink, lpath))
	code = buf.ReadByteAt(1)
	str, ok := buf.ReadString(2)
	buf.Done()
	bufPool.Put(buf)

	if code == wsfsprotocol.ErrorOK {
		if ok {
			return "", wsfsprotocol.ErrorIO
		}
		return str, code
	} else {
		return "", code
	}
}

const maxWritePayload int = maxFrameSize - 1 - 1 - 4

func (s *Session) CmdWrite(fd uint32, data []byte) (written uint64, code uint8) {
	if len(data) < maxWritePayload {
		buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdWrite, fd, data))
		code = buf.ReadByteAt(1)
		if code != wsfsprotocol.ErrorOK {
			buf.Done()
			bufPool.Put(buf)
			return 0, code
		}

		if !buf.Ensure(9) {
			log.Error().Msg("Command response too short")
			buf.Done()
			bufPool.Put(buf)
			return 0, wsfsprotocol.ErrorIO
		}
		written = buf.ReadU64(2)
		buf.Done()
		bufPool.Put(buf)
		return
	} else {
		off := 0
		for range len(data) / maxWritePayload {
			buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdWrite, fd, data[off:off+maxWritePayload]))
			code = buf.ReadByteAt(1)
			if code != wsfsprotocol.ErrorOK {
				buf.Done()
				bufPool.Put(buf)
				return uint64(off), code
			}

			if !buf.Ensure(9) {
				log.Error().Msg("Command response too short")
				buf.Done()
				bufPool.Put(buf)
				return 0, wsfsprotocol.ErrorIO
			}
			off += int(buf.ReadU64(2))
			buf.Done()
			bufPool.Put(buf)
		}
		if len(data)%maxWritePayload == 0 {
			return uint64(off), wsfsprotocol.ErrorOK
		}

		buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdWrite, fd, data[off:off+len(data)%maxWritePayload]))
		code = buf.ReadByteAt(1)
		if code != wsfsprotocol.ErrorOK {
			buf.Done()
			bufPool.Put(buf)
			return uint64(off), code
		}

		if !buf.Ensure(9) {
			log.Error().Msg("Command response too short")
			buf.Done()
			bufPool.Put(buf)
			return 0, wsfsprotocol.ErrorIO
		}
		off += int(buf.ReadU64(2))
		buf.Done()
		bufPool.Put(buf)

		return uint64(off), code
	}
}

func (s *Session) CmdSeek(fd uint32, flag uint32, off int64) (pos uint64, code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdSeek, fd, flag, off))
	code = buf.ReadByteAt(1)

	if code != wsfsprotocol.ErrorOK {
		buf.Done()
		bufPool.Put(buf)
		return
	}

	if !buf.Ensure(9) {
		buf.Done()
		bufPool.Put(buf)
		log.Error().Msg("Command response too short")
		return 0, wsfsprotocol.ErrorIO
	}

	pos = buf.ReadU64(2)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdAllocate(fd uint32, flag uint32, off uint64, size uint64) uint8 {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdSeek, fd, flag, off, size))
	code := buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return code
}

type FileInfo struct {
	Size  uint64
	MTime int64
	Mode  uint32
	Owner uint8
}

func FileInfoFromDirItem(di *DirItem) FileInfo {
	return FileInfo{
		Size:  di.Size,
		MTime: di.MTime,
		Mode:  di.Mode,
		Owner: di.Owner,
	}
}

func (s *Session) CmdGetAttr(fpath string) (fi FileInfo, code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdGetAttr, fpath))
	code = buf.ReadByteAt(1)
	if code != wsfsprotocol.ErrorOK {
		buf.Done()
		bufPool.Put(buf)
		return
	}

	if !buf.Ensure(22) {
		buf.Done()
		bufPool.Put(buf)
		log.Error().Msg("Command response too short")
		code = wsfsprotocol.ErrorIO
		return
	}

	fi.Size = buf.ReadU64(2)
	fi.MTime = int64(buf.ReadU64(10))
	fi.Mode = buf.ReadU32(18)
	fi.Owner = buf.ReadByteAt(22)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdSetAttr(fpath string, flag uint8, fi FileInfo) (code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdSetAttr, fpath, flag, fi.Size, fi.MTime, fi.Mode, fi.Owner))
	code = buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdSync(fd uint32) (code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdSync, fd))
	code = buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdMkdir(fpath string, mode uint32) (code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdMkdir, fpath, mode))
	code = buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdSymLink(target string, fpath string) (code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdSymLink, target, fpath))
	code = buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdRemove(fpath string) (code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdRemove, fpath))
	code = buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdRmDir(fpath string) (code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdRmDir, fpath))
	code = buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return
}

type FsInfo struct {
	Total     uint64
	Free      uint64
	Available uint64
}

func (s *Session) CmdFsStat(fpath string) (fsi FsInfo, code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdFsStat, fpath))
	code = buf.ReadByteAt(1)
	if code != wsfsprotocol.ErrorOK {
		buf.Done()
		bufPool.Put(buf)
		return
	}

	if !buf.Ensure(25) {
		buf.Done()
		bufPool.Put(buf)
		log.Error().Msg("Command response too short")
		code = wsfsprotocol.ErrorIO
		return
	}

	fsi.Total = buf.ReadU64(2)
	fsi.Free = buf.ReadU64(10)
	fsi.Available = buf.ReadU64(18)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdReadAt(fd uint32, offset uint64, dest []byte) (uint64, uint8) {
	clientMark := s.newClientMark()
	s.writeRequest <- msg(clientMark, wsfsprotocol.CmdReadAt, fd, offset, uint64(len(dest)))

	var off int
	for {
		rsp := <-s.readRequests[clientMark]

		code := rsp.ReadByteAt(1)
		data := rsp.Done()[2:]
		copy(dest[off:], data)
		bufPool.Put(rsp)

		if code == wsfsprotocol.ErrorPartialResponse {
			off += len(data)
			continue
		} else {
			s.clientMarks[clientMark].Unlock()
			return uint64(off + len(data)), code
		}
	}
}

const maxWriteAtPayload int = maxFrameSize - 1 - 1 - 4 - 8

func (s *Session) CmdWriteAt(fd uint32, offset uint64, data []byte) (written uint64, code uint8) {
	if len(data) < maxWriteAtPayload {
		buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdWriteAt, fd, offset, data))
		code = buf.ReadByteAt(1)
		if code != wsfsprotocol.ErrorOK {
			buf.Done()
			bufPool.Put(buf)
			return 0, code
		}

		if !buf.Ensure(9) {
			log.Error().Msg("Command response too short")
			buf.Done()
			bufPool.Put(buf)
			return 0, wsfsprotocol.ErrorIO
		}
		written = buf.ReadU64(2)
		buf.Done()
		bufPool.Put(buf)
		return
	} else {
		off := 0
		for range len(data) / maxWriteAtPayload {
			buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdWriteAt, fd, offset+uint64(off), data[off:off+maxWriteAtPayload]))
			code = buf.ReadByteAt(1)
			if code != wsfsprotocol.ErrorOK {
				buf.Done()
				bufPool.Put(buf)
				return uint64(off), code
			}

			if !buf.Ensure(9) {
				log.Error().Msg("Command response too short")
				buf.Done()
				bufPool.Put(buf)
				return 0, wsfsprotocol.ErrorIO
			}
			off += int(buf.ReadU64(2))
			buf.Done()
			bufPool.Put(buf)
		}
		if len(data)%maxWriteAtPayload == 0 {
			return uint64(off), wsfsprotocol.ErrorOK
		}

		buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdWriteAt, fd, offset+uint64(off), data[off:off+len(data)%maxWriteAtPayload]))
		code = buf.ReadByteAt(1)
		if code != wsfsprotocol.ErrorOK {
			buf.Done()
			bufPool.Put(buf)
			return uint64(off), code
		}

		if !buf.Ensure(9) {
			log.Error().Msg("Command response too short")
			buf.Done()
			bufPool.Put(buf)
			return 0, wsfsprotocol.ErrorIO
		}
		off += int(buf.ReadU64(2))
		buf.Done()
		bufPool.Put(buf)

		return uint64(off), code
	}
}

func (s *Session) CmdRename(old string, new string, mode uint32) (code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdRename, old, new, mode))
	code = buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdCopyFileRange(wfd1 uint32, wfd2 uint32, off1 uint64, off2 uint64, size uint64) (copyed uint64, code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdCopyFileRange, wfd1, wfd2, off1, off2, size))
	if !buf.Ensure(9) {
		buf.Done()
		bufPool.Put(buf)
		return 0, wsfsprotocol.ErrorIO
	}
	copyed = buf.ReadU64(2)
	buf.Done()
	bufPool.Put(buf)
	return
}

func (s *Session) CmdSetAttrByFD(wfd uint32, flag uint8, fi FileInfo) (code uint8) {
	buf := s.execCommand(msg(s.newClientMark(), wsfsprotocol.CmdSetAttrByFD, wfd, flag, fi.Size, fi.MTime, fi.Mode, fi.Owner))
	code = buf.ReadByteAt(1)
	buf.Done()
	bufPool.Put(buf)
	return
}

func GetDirTreeItemAtDepth(tree *DirItem, depth *uint8) *DirItem {
	for range *depth {
		tree = &tree.Child[len(tree.Child)-1]
	}
	return tree
}

func readTree(data *util.Buffer, stack *[]*[]DirItem) (ok bool) {
	ok = true
	off := 1

	current := (*stack)[len(*stack)-1]
	var tmp DirItem
	for {
		off += 1
		if !data.Ensure(off) {
			break
		}
		indicator := uint8(data.ReadByteAt(off))

		switch indicator {
		case wsfsprotocol.TREEDIR_INDICATOR_ENTER_DIR:
			(*current)[len(*current)-1].Child = []DirItem{}
			*stack = append(*stack, &((*current)[len(*current)-1].Child))
			current = (*stack)[len(*stack)-1]
			continue
		case wsfsprotocol.TREEDIR_INDICATOR_END_DIR:
			*stack = (*stack)[:len(*stack)-1]
			current = (*stack)[len(*stack)-1]
			continue
		case wsfsprotocol.TREEDIR_INDICATOR_END_DIR_WITH_FAIL:
			*stack = (*stack)[:len(*stack)-1]
			(*current) = nil
			current = (*stack)[len(*stack)-1]
			continue
		default:
		}

		off += 1
		if ok = data.Ensure(off); !ok {
			break
		}
		tmp.Name, ok = data.ReadString(off)
		if !ok {
			return
		}
		off += len(tmp.Name) + 1

		ok = data.Ensure(off + 20)
		if !ok {
			return
		}
		tmp.Size = data.ReadU64(off)
		tmp.MTime = int64(data.ReadU64(off + 8))
		tmp.Mode = data.ReadU32(off + 8 + 8)
		tmp.Owner = data.ReadByteAt(off + 8 + 8 + 4)
		off += 20
		tmp.Child = nil

		if indicator == wsfsprotocol.TREEDIR_INDICATOR_FILE_WITH_DATA {
			ok = data.Ensure(off + int(tmp.Size))
			if !ok {
				return
			}
			tmp.Data = data.ReadDataAtLen(off, int(tmp.Size))
			off += int(tmp.Size)
		} else {
			//tmp.Data = nil
		}

		*current = append(*current, tmp)
	}

	return
}

func (s *Session) CmdTreeDir(path string, depth uint8, hint string) (tree []DirItem, code uint8) {
	clientMark := s.newClientMark()
	s.writeRequest <- msg(clientMark, wsfsprotocol.CmdTreeDir, path, depth, hint)

	var rsp *util.Buffer
	tree = append(tree, DirItem{})
	stack := []*[]DirItem{&tree} // stack store where sub tree should return to
	for {
		rsp = <-s.readRequests[clientMark]

		code = rsp.ReadByteAt(1)
		if code != wsfsprotocol.ErrorOK &&
			code != wsfsprotocol.ErrorPartialResponse {
			rsp.Done()
			bufPool.Put(rsp)
			return
		}

		ok := readTree(rsp, &stack)
		rsp.Done()
		bufPool.Put(rsp)
		if !ok {
			log.Error().Msg("Command response too short")
			s.clientMarks[clientMark].Unlock()
			return nil, wsfsprotocol.ErrorIO
		}

		if code == wsfsprotocol.ErrorPartialResponse {
			continue
		} else {
			break
		}
	}

	if len(tree) != 1 || tree[0].Child == nil {
		code = wsfsprotocol.ErrorIO
	}
	tree = tree[0].Child

	s.clientMarks[clientMark].Unlock()
	return
}
