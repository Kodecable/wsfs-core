package session

import (
	"bytes"
	"os"
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/rs/zerolog/log"
)

type DirItem struct {
	Name  string
	Size  uint64
	MTime wsfsprotocol.Timespec
	Mode  uint32
	Owner uint8
	Child []DirItem
	Data  []byte

	// childReady is non-nil iff this entry has an asynchronous CmdReadDirPlus
	// prefetch result pending. Readers block on this channel before using
	// Child; the background goroutine closes it once the final Child state is
	// known.
	//
	// Child semantics after childReady is closed:
	//   nil             - no cached result, fall back to network
	//   []DirItem{}     - cached empty directory result
	//   []DirItem{...}  - cached non-empty directory result
	childReady chan struct{}
}

// WaitPrefetch blocks until the background goroutine has finished resolving
// this entry's prefetched Child state.
func (d *DirItem) WaitPrefetch() {
	if d.childReady != nil {
		<-d.childReady
	}
}

// PrefetchReady returns the channel closed when this entry's pending prefetch
// result is resolved, or nil if there is no pending prefetch.
func (d DirItem) PrefetchReady() <-chan struct{} {
	return d.childReady
}

const maxWritePayload int = maxFrameSize - 6 // header(2) + FD(4)

func appendDirItemsFromReader(list []DirItem, r *bytes.Reader) ([]DirItem, error) {
	for r.Len() > 0 {
		var ent wsfsprotocol.Dirent
		if err := wsfsprotocol.ReadDirentFromReader(&ent, r); err != nil {
			return list, err
		}
		list = append(list, makeDirItemFromDirent(&ent))
	}
	return list, nil
}

func readRspErrorDesc(data []byte) string {
	var rsp wsfsprotocol.RspError
	if err := wsfsprotocol.ReadRspErrorFromReader(&rsp, bytes.NewReader(data)); err != nil {
		log.Error().Err(err).Msg("Failed to decode error response")
		return ""
	}
	return rsp.Desc
}

const (
	sectionRoot     = 0
	sectionPrefetch = 1
)

type readDirPlusResult struct {
	list []DirItem
	code uint8
}

func makeDirItemFromDirent(d *wsfsprotocol.Dirent) DirItem {
	return DirItem{
		Name:  d.Name,
		Size:  d.Size,
		MTime: d.MTime,
		Mode:  d.Mode,
		Owner: d.Owner,
	}
}

// setChild populates list[idx].Child with a cached directory result and closes
// childReady so waiters can observe the final Child value.
func setChild(list []DirItem, idx int, items []DirItem) {
	if items == nil {
		items = []DirItem{}
	}
	list[idx].Child = items
	if list[idx].childReady != nil {
		close(list[idx].childReady)
	}
}

// closeChildReady closes the childReady channel for the given entry without
// populating Child, leaving Child == nil to mean "no cached result".
func closeChildReady(list []DirItem, idx int) {
	if list[idx].childReady != nil {
		close(list[idx].childReady)
	}
}

// closeAllRemainingChildReady closes every childReady channel that was
// created for directory entries but never explicitly resolved (neither
// via setChild nor via closedChildReady).  This must be called once,
// at the end of the response stream, so that no Lookup goroutine blocks
// indefinitely.
//
// IMPORTANT: The caller (parseReadDirPlus) holds a reference to the
// backing array of list via the same slice header it already sent
// through rootCh.  After rootCh delivery the goroutine only *mutates*
// existing fields (Child, childReady) – it never appends to list, so the
// backing array is shared safely with the receiver.
func closeAllRemainingChildReady(list []DirItem, pendingDirs []int) {
	for _, idx := range pendingDirs {
		closeChildReady(list, idx)
	}
}

// parseReadDirPlus is a two-phase parser:
//
//	Phase 1 — process ROOT-section messages, build list.  As soon as the
//	  parser leaves the ROOT section (first PREFETCH indicator or
//	  ErrorOK), it sends list+code through rootCh so that CmdReadDirPlus
//	  can return immediately.
//	Phase 2 — continue processing PREFETCH messages in the background.
//	  For each successfully prefetched directory, Child is populated
//	  and childReady is closed; for skipped or missing directories only
//	  childReady is closed.
func (s *Session) parseReadDirPlus(
	clientMark uint8,
	rootCh chan<- readDirPlusResult,
) {
	section := sectionRoot
	var list []DirItem
	var items []DirItem
	currentPrefetchIdx := -1
	rootSent := false
	pendingDirs := make([]int, 0)

	defer func() {
		s.marks[clientMark].Unlock()
	}()

	readDirent := func(r *bytes.Reader) (wsfsprotocol.Dirent, bool) {
		var d wsfsprotocol.Dirent
		if err := wsfsprotocol.ReadDirentFromReader(&d, r); err != nil {
			log.Error().Err(err).Msg("Failed to read dirent in ReadDirPlus")
			if currentPrefetchIdx >= 0 {
				closeChildReady(list, currentPrefetchIdx)
			}
			closeAllRemainingChildReady(list, pendingDirs)
			if !rootSent {
				rootCh <- readDirPlusResult{list: nil, code: wsfsprotocol.ErrorIO}
			}
			return wsfsprotocol.Dirent{}, false
		}
		return d, true
	}

	nextPendingDir := func() int {
		if len(pendingDirs) == 0 {
			return -1
		}
		idx := pendingDirs[0]
		pendingDirs = pendingDirs[1:]
		return idx
	}

	flushPrefetch := func(commit bool) {
		if section != sectionPrefetch || currentPrefetchIdx < 0 {
			return
		}
		if commit {
			setChild(list, currentPrefetchIdx, items)
		} else {
			closeChildReady(list, currentPrefetchIdx)
		}
		items = nil
		currentPrefetchIdx = -1
	}

	for {
		rsp := <-s.responses[clientMark]
		code := rsp.Bytes[1]

		if code != wsfsprotocol.ErrorOK &&
			code != wsfsprotocol.ErrorPartialResponse {
			flushPrefetch(false)
			closeAllRemainingChildReady(list, pendingDirs)
			bufPool.Put(rsp)
			if !rootSent {
				rootCh <- readDirPlusResult{list: nil, code: code}
			}
			return
		}

		r := bytes.NewReader(rsp.Bytes[2:rsp.Writted()])

		for r.Len() > 0 {
			indicator, err := r.ReadByte()
			if err != nil {
				log.Error().Err(err).Msg("Failed to read indicator in ReadDirPlus")
				flushPrefetch(false)
				closeAllRemainingChildReady(list, pendingDirs)
				bufPool.Put(rsp)
				if !rootSent {
					rootCh <- readDirPlusResult{list: nil, code: wsfsprotocol.ErrorIO}
				}
				return
			}

			switch indicator {
			case wsfsprotocol.READDIRPLUS_INDICATOR_CONTINUE:
				d, ok := readDirent(r)
				if !ok {
					bufPool.Put(rsp)
					return
				}
				di := makeDirItemFromDirent(&d)
				if section == sectionRoot {
					if d.Mode&uint32(os.ModeDir) != 0 {
						di.childReady = make(chan struct{})
						pendingDirs = append(pendingDirs, len(list))
					}
					list = append(list, di)
				} else {
					items = append(items, di)
				}
			case wsfsprotocol.READDIRPLUS_INDICATOR_PREFETCH:
				if !rootSent {
					rootCh <- readDirPlusResult{list: list, code: wsfsprotocol.ErrorOK}
					rootSent = true
				}
				flushPrefetch(true)
				section = sectionPrefetch
				currentPrefetchIdx = nextPendingDir()
				if currentPrefetchIdx < 0 {
					log.Error().Msg("ReadDirPlus prefetch ordering mismatch")
					closeAllRemainingChildReady(list, pendingDirs)
					bufPool.Put(rsp)
					if !rootSent {
						rootCh <- readDirPlusResult{list: nil, code: wsfsprotocol.ErrorIO}
					}
					return
				}
			case wsfsprotocol.READDIRPLUS_INDICATOR_PREFETCH_SKIP:
				if !rootSent {
					rootCh <- readDirPlusResult{list: list, code: wsfsprotocol.ErrorOK}
					rootSent = true
				}
				flushPrefetch(false)
				section = sectionPrefetch
				currentPrefetchIdx = nextPendingDir()
				if currentPrefetchIdx < 0 {
					log.Error().Msg("ReadDirPlus prefetch skip ordering mismatch")
					closeAllRemainingChildReady(list, pendingDirs)
					bufPool.Put(rsp)
					if !rootSent {
						rootCh <- readDirPlusResult{list: nil, code: wsfsprotocol.ErrorIO}
					}
					return
				}
			default:
				log.Error().Uint8("Indicator", indicator).Msg("Unknown ReadDirPlus indicator")
				flushPrefetch(false)
				closeAllRemainingChildReady(list, pendingDirs)
				bufPool.Put(rsp)
				if !rootSent {
					rootCh <- readDirPlusResult{list: nil, code: wsfsprotocol.ErrorIO}
				}
				return
			}
		}
		bufPool.Put(rsp)

		if code == wsfsprotocol.ErrorOK {
			flushPrefetch(true)
			closeAllRemainingChildReady(list, pendingDirs)
			if !rootSent {
				rootCh <- readDirPlusResult{list: list, code: wsfsprotocol.ErrorOK}
			}
			return
		}
	}
}

func (s *Session) CmdReadDirPlus(path string) (list []DirItem, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdReadDirPlus) {
		s.marks[clientMark].Unlock()
		return nil, wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdReadDirPlusStructToWriter(wsfsprotocol.CmdReadDirPlusStruct{Path: path}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return nil, wsfsprotocol.ErrorIO
	}

	rootCh := make(chan readDirPlusResult, 1)
	go func() {
		s.parseReadDirPlus(clientMark, rootCh)
	}()

	result := <-rootCh
	return result.list, result.code
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

		var readErr error
		list, readErr = appendDirItemsFromReader(list, bytes.NewReader(rsp.Bytes[2:rsp.Writted()]))
		bufPool.Put(rsp)
		if readErr != nil {
			log.Error().Err(readErr).Msg("Failed to read directory entries")
			s.marks[clientMark].Unlock()
			return nil, wsfsprotocol.ErrorIO
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

func (s *Session) CmdSeek(fd uint32, whence uint8, off int64) (pos uint64, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdSeek) {
		s.marks[clientMark].Unlock()
		return
	}
	err := wsfsprotocol.WriteCmdSeekStructToWriter(wsfsprotocol.CmdSeekStruct{FD: fd, Whence: whence, Offset: off}, s.writer)
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

func (s *Session) CmdCloneFileRange(wfd1 uint32, wfd2 uint32, off1 uint64, off2 uint64, size uint64) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdCloneFileRange) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdCloneFileRangeStructToWriter(wsfsprotocol.CmdCloneFileRangeStruct{
		SrcFD: wfd1, DstFD: wfd2, SrcOffset: off1, DstOffset: off2, Size: size,
	}, s.writer)
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

func (s *Session) CmdGetFileLock(fd uint32, fileLock wsfsprotocol.FileLockInfo) (out wsfsprotocol.FileLockInfo, code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdGetFileLock) {
		s.marks[clientMark].Unlock()
		return out, wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdGetFileLockStructToWriter(wsfsprotocol.CmdGetFileLockStruct{FD: fd, FileLock: fileLock}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return out, wsfsprotocol.ErrorIO
	}

	rspBuf := <-s.responses[clientMark]
	s.marks[clientMark].Unlock()
	code = rspBuf.Bytes[1]
	if code != wsfsprotocol.ErrorOK {
		bufPool.Put(rspBuf)
		return out, code
	}

	var rsp wsfsprotocol.RspGetFileLock
	err = wsfsprotocol.ReadRspGetFileLockFromReader(&rsp, bytes.NewReader(rspBuf.Bytes[2:rspBuf.Writted()]))
	bufPool.Put(rspBuf)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode CmdGetFileLock response")
		return out, wsfsprotocol.ErrorIO
	}
	return rsp.FileLock, code
}

func (s *Session) CmdSetFileLock(fd uint32, fileLock wsfsprotocol.FileLockInfo) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdSetFileLock) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdSetFileLockStructToWriter(wsfsprotocol.CmdSetFileLockStruct{FD: fd, FileLock: fileLock}, s.writer)
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

func (s *Session) CmdSetFileLockWait(fd uint32, fileLock wsfsprotocol.FileLockInfo) (code uint8) {
	clientMark := s.newClientMark()

	if !s.beginRequest(clientMark, wsfsprotocol.CmdSetFileLockWait) {
		s.marks[clientMark].Unlock()
		return wsfsprotocol.ErrorIO
	}
	err := wsfsprotocol.WriteCmdSetFileLockWaitStructToWriter(wsfsprotocol.CmdSetFileLockWaitStruct{FD: fd, FileLock: fileLock}, s.writer)
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
