package wsfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog/log"
)

//go:generate go run genCommandCalls.go -- "../../share/wsfsprotocol/const.go" "commandCalls.go" "wsfs"

const (
	MaxMsgSize            = wsfsprotocol.MaxMsgSize
	maxReadPayLoad uint64 = uint64(MaxMsgSize) - 2
)

var (
	bufPool = sync.Pool{
		New: func() any {
			return util.NewBuffer(MaxMsgSize)
		},
	}
)

func putBuf(buf *util.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}

func (s *session) dispatchCommand(r io.Reader) (err error) {
	var header [2]byte
	_, err = io.ReadFull(r, header[:])
	if err != nil {
		return fmt.Errorf("bad command header: %e", err)
	}
	//log.Debug().Uint8("Cm", header[0]).Uint8("Op", header[1]).Msg("Recived commnad")
	s.doCommandCall(header[0], header[1], r)
	// coder/websocket requires the current message reader to be drained to EOF
	// before Reader can be called for the next message.
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return fmt.Errorf("bad command payload: %w", err)
	}
	if n > 0 {
		log.Warn().
			Uint8("Cm", header[0]).
			Uint8("Op", header[1]).
			Int64("TrailingBytes", n).
			Msg("Command payload not fully consumed")
	}
	return nil
}

func osErrCode(err error) uint8 {
	if code, ok := osErrCode_osOverride(err); ok {
		return code
	}
	if os.IsExist(err) {
		return wsfsprotocol.ErrorExists
	} else if os.IsNotExist(err) {
		return wsfsprotocol.ErrorNotExists
	} else if os.IsPermission(err) {
		return wsfsprotocol.ErrorAccessRestricted
	} else {
		return wsfsprotocol.ErrorUnknown
	}
}

// base ends with "/"
func (s *session) restrictingSymlinkByFileInfo(base string, fi fs.FileInfo) (fs.FileInfo, error) {
	target, err := os.Readlink(base + fi.Name())
	if err != nil {
		return fi, err
	}

	if target[0] != '/' {
		target = path.Clean(base + target)
	}

	if strings.HasPrefix(target, s.storage.Path) {
		return fi, nil
	} else {
		// We stat(base + fi.Name()) here ranther stat(target)
		// Consider this situation:
		//   /
		//   ├─ A
		//   │  └─ B (link to /C)
		//   ├─ C
		//   │  └─ D (link to ../E)
		//   └─ E
		// If base is B and fi is D, var target will point to /A/B/E which not exists,
		// but this file is actually /E which do exists.
		return os.Stat(base + fi.Name())
	}
}

func (s *session) lookupDirent(base string, dirent fs.DirEntry) (wdirent wsfsprotocol.Dirent, err error) {
	wdirent.Name = dirent.Name()
	fi, err := dirent.Info()
	if err != nil {
		return
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		fi, err = s.restrictingSymlinkByFileInfo(base, fi)
		if err != nil {
			return
		}
	}
	wdirent.Size = uint64(fi.Size())
	wdirent.MTime = fi.ModTime().Unix()
	wdirent.Mode = uint32(fi.Mode())
	wdirent.Owner = s.convOwner(fi)
	return
}

func (s *session) getAttrFileInfo(apath string) (fs.FileInfo, error) {
	fi, err := os.Lstat(apath)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&fs.ModeSymlink == 0 {
		return fi, nil
	}
	return s.restrictingSymlinkByFileInfo(filepath.Dir(apath)+"/", fi)
}

func (s *session) cmdReadDir(clientMark uint8, req wsfsprotocol.CmdReadDirStruct) {
	path := req.Path

	if !util.IsUrlValid(path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + path

	f, err := os.Open(apath)
	if err != nil {
		s.writeRspError(clientMark, osErrCode(err), "open dir failed")
		return
	}
	defer func() {
		if f.Close() != nil {
			log.Error().Err(err).Str("Path", apath).Msg("close dir failed")
		}
	}()

	rsp := bufPool.Get().(*util.Buffer)
	defer putBuf(rsp)
	rsp.Write([]byte{clientMark, wsfsprotocol.ErrorPartialResponse})
	for {
		dirents, readdirerr := f.ReadDir(16)
		if readdirerr != nil && !errors.Is(readdirerr, io.EOF) {
			s.writeRspError(clientMark, osErrCode(readdirerr), "read dir failed")
			break
		}

		for _, dirent := range dirents {
			wdirent, lookupErr := s.lookupDirent(apath, dirent)
			if lookupErr != nil {
				wdirent = wsfsprotocol.Dirent{
					Name:  dirent.Name(),
					Mode:  uint32(os.ModeIrregular),
					Owner: wsfsprotocol.OWNER_NN,
				}
			}

			if rsp.Writted()+wsfsprotocol.GetDirentRequiredSize(wdirent) > wsfsprotocol.MaxMsgSize {
				s.write(rsp.Done())
				rsp.Write([]byte{clientMark, wsfsprotocol.ErrorPartialResponse})
			}
			wsfsprotocol.WriteDirentToWriter(wdirent, rsp)
		}

		if len(dirents) == 0 || errors.Is(readdirerr, io.EOF) {
			rsp.Bytes[1] = wsfsprotocol.ErrorOK
			s.write(rsp.Done())
			break
		}
	}
}

func (s *session) cmdGetAttr(clientMark uint8, req wsfsprotocol.CmdGetAttrStruct) {
	lpath := req.Path

	if !util.IsUrlValid(lpath) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	fi, err := s.getAttrFileInfo(apath)
	if err != nil {
		goto BAD
	}
	if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		err = wsfsprotocol.WriteRspGetAttrToWriter(wsfsprotocol.RspGetAttr{FI: wsfsprotocol.FileInfo{
			Size:  uint64(fi.Size()),
			MTime: fi.ModTime().Unix(),
			Mode:  uint32(fi.Mode()),
			Owner: s.convOwner(fi),
		}}, s.writer)
		s.writeDone(err)
	}
	return
BAD:
	s.writeRspError(clientMark, osErrCode(err), "syscall error")
}

func (s *session) lookupDirentSafe(apath string, entry fs.DirEntry) wsfsprotocol.Dirent {
	wdirent, err := s.lookupDirent(apath, entry)
	if err != nil {
		return wsfsprotocol.Dirent{
			Name:  entry.Name(),
			Mode:  uint32(os.ModeIrregular),
			Owner: wsfsprotocol.OWNER_NN,
		}
	}
	return wdirent
}

func (s *session) writeDirentChunk(rsp *util.Buffer, clientMark uint8, wdirent wsfsprotocol.Dirent) {
	requiredSize := 1 + wsfsprotocol.GetDirentRequiredSize(wdirent)
	if rsp.Writted()+requiredSize > MaxMsgSize {
		s.write(rsp.Done())
		rsp.Write([]byte{clientMark, wsfsprotocol.ErrorPartialResponse})
	}
	rsp.Write([]byte{wsfsprotocol.READDIRPLUS_INDICATOR_CONTINUE})
	wsfsprotocol.WriteDirentToWriter(wdirent, rsp)
}

func (s *session) writePrefetchIndicator(rsp *util.Buffer, clientMark uint8, indicator uint8) {
	requiredSize := 1
	if rsp.Writted()+requiredSize > MaxMsgSize {
		s.write(rsp.Done())
		rsp.Write([]byte{clientMark, wsfsprotocol.ErrorPartialResponse})
	}
	rsp.Write([]byte{indicator})
}

type prefetchDirState struct {
	absPath string
	file    *os.File
	pending []fs.DirEntry
	count   int
}

func (s *session) preparePrefetchDir(basePath string, entry fs.DirEntry) (*prefetchDirState, error) {
	childAbsPath := basePath + entry.Name() + "/"
	cf, err := os.Open(childAbsPath)
	if err != nil {
		return nil, err
	}

	entries, readErr := cf.ReadDir(16)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		cf.Close()
		return nil, readErr
	}
	return &prefetchDirState{
		absPath: childAbsPath,
		file:    cf,
		pending: entries,
		count:   0,
	}, nil
}

func (s *session) nextPrefetchDir(first []fs.DirEntry, basePath string, start int, used *int) (*prefetchDirState, int, error) {
	for i := start; i < len(first); i++ {
		if *used >= maxPrefetchDirs {
			return nil, i, nil
		}
		if !first[i].IsDir() {
			continue
		}
		*used++
		state, err := s.preparePrefetchDir(basePath, first[i])
		if err != nil {
			return nil, i + 1, err
		}
		return state, i + 1, nil
	}
	return nil, len(first), nil
}

func (s *session) streamPrefetchDir(rsp *util.Buffer, clientMark uint8, state *prefetchDirState) (failed bool) {
	defer state.file.Close()

	for {
		for _, entry := range state.pending {
			state.count++
			if state.count > maxPrefetchDirEntries {
				return true
			}
			s.writeDirentChunk(rsp, clientMark, s.lookupDirentSafe(state.absPath, entry))
		}

		entries, err := state.file.ReadDir(16)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return false
			}
			return true
		}
		state.pending = entries
	}
}

const (
	maxRootEntriesForPrefetch = 500
	maxPrefetchDirEntries     = 1000
	maxPrefetchDirs           = 32
)

func (s *session) cmdReadDirPlus(clientMark uint8, req wsfsprotocol.CmdReadDirPlusStruct) {
	path := req.Path

	if !util.IsUrlValid(path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + path

	f, err := os.Open(apath)
	if err != nil {
		s.writeRspError(clientMark, osErrCode(err), "open dir failed")
		return
	}
	defer f.Close()

	// 探测根目录是否超出 prefetch 门槛；ROOT 结果始终完整返回。
	first, err := f.ReadDir(maxRootEntriesForPrefetch + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		s.writeRspError(clientMark, osErrCode(err), "read dir failed")
		return
	}
	disablePrefetch := len(first) > maxRootEntriesForPrefetch

	// 发送第一批 ROOT 条目
	rsp := bufPool.Get().(*util.Buffer)
	rsp.Write([]byte{clientMark, wsfsprotocol.ErrorPartialResponse})
	for _, entry := range first {
		s.writeDirentChunk(rsp, clientMark, s.lookupDirentSafe(apath, entry))
	}

	// 继续读剩余 ROOT 条目
	for {
		more, err := f.ReadDir(16)
		for _, entry := range more {
			s.writeDirentChunk(rsp, clientMark, s.lookupDirentSafe(apath, entry))
		}
		if err != nil && !errors.Is(err, io.EOF) {
			putBuf(rsp)
			s.writeRspError(clientMark, osErrCode(err), "read dir failed")
			return
		}
		if errors.Is(err, io.EOF) || len(more) == 0 {
			break
		}
	}

	// 结束 ROOT 段
	if disablePrefetch {
		rsp.Bytes[1] = wsfsprotocol.ErrorOK
		s.write(rsp.Done())
		putBuf(rsp)
		return
	}
	s.write(rsp.Done())
	putBuf(rsp)

	// Prefetch child directories.
	//
	// Response framing semantics:
	// - ROOT entries are streamed first as CONTINUE records.
	// - PREFETCH starts a new child-directory prefetch section for the next
	//   directory entry from the ROOT result, in ROOT order.
	// - PREFETCH_SKIP means the current prefetch section is invalid and must be
	//   discarded; the next prefetch section starts immediately for the next
	//   directory entry from the ROOT result, again in ROOT order.
	// - PREFETCH and PREFETCH_SKIP are pure section markers. They do not carry a
	//   directory name or an implicit first dirent.
	// - An empty directory is represented naturally: PREFETCH begins its section,
	//   then the next indicator arrives without any CONTINUE records in between.
	prefetchCount := 0
	for idx := 0; idx < len(first) && prefetchCount < maxPrefetchDirs; {
		state, nextIdx, err := s.nextPrefetchDir(first, apath, idx, &prefetchCount)
		idx = nextIdx
		if err != nil {
			s.writeRspError(clientMark, wsfsprotocol.ErrorIO, "read prefetch dir failed")
			return
		}
		if state == nil {
			continue
		}

		rsp := bufPool.Get().(*util.Buffer)
		rsp.Write([]byte{clientMark, wsfsprotocol.ErrorPartialResponse})
		s.writePrefetchIndicator(rsp, clientMark, wsfsprotocol.READDIRPLUS_INDICATOR_PREFETCH)

		for {
			if !s.streamPrefetchDir(rsp, clientMark, state) {
				s.write(rsp.Done())
				putBuf(rsp)
				break
			}

			nextState, newIdx, err := s.nextPrefetchDir(first, apath, idx, &prefetchCount)
			idx = newIdx
			if err != nil || nextState == nil {
				putBuf(rsp)
				s.writeRspError(clientMark, wsfsprotocol.ErrorIO, "read prefetch dir failed")
				return
			}

			s.writePrefetchIndicator(rsp, clientMark, wsfsprotocol.READDIRPLUS_INDICATOR_PREFETCH_SKIP)
			state = nextState
		}
	}

	// 最终 ErrorOK
	s.writeRspOK(clientMark)
}

func (s *session) cmdWriteStreamOpen(clientMark uint8, req wsfsprotocol.CmdWriteStreamOpenStruct) {
	stream := &writeStreamState{
		offset: req.Offset,
	}
	if _, loaded := s.writeStreams.LoadOrStore(clientMark, stream); loaded {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "write stream already open")
		return
	}

	if rsfd, ok := s.fds.Load(req.FD); ok {
		stream.fd = rsfd.(sfd_t)
	} else {
		s.markWriteStreamError(clientMark, stream, wsfsprotocol.ErrorInvailFD, "bad fd")
	}

	if len(req.Data) > 0 {
		s.writeStreamChunk(clientMark, stream, req.Data)
	}
}

func (s *session) cmdWriteStreamData(clientMark uint8, req wsfsprotocol.CmdWriteStreamDataStruct) {
	stream, ok := s.loadWriteStream(clientMark)
	if !ok {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "write stream not open")
		return
	}

	if len(req.Data) > 0 {
		s.writeStreamChunk(clientMark, stream, req.Data)
	}

	if req.IsEnd != 0 {
		s.writeStreams.Delete(clientMark)
		s.writeRspWriteStreamClose(clientMark, stream.written)
	}
}

func (s *session) markWriteStreamError(clientMark uint8, stream *writeStreamState, code uint8, desc string) {
	if stream.writeErrSent {
		return
	}
	stream.writeErrSent = true
	s.writeRspError(clientMark, code, desc)
}

func (s *session) writeRspWriteStreamClose(clientMark uint8, written uint64) {
	if !s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		return
	}
	err := wsfsprotocol.WriteRspWriteStreamCloseToWriter(wsfsprotocol.RspWriteStreamClose{Written: written}, s.writer)
	s.writeDone(err)
}
