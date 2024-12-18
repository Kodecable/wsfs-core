package wsfs

import (
	"encoding/binary"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"

	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog/log"
)

const (
	maxFrameSize          = wsfsprotocol.MaxResponseLength
	maxReadPayLoad uint64 = uint64(maxFrameSize) - 2
)

var (
	bufPool = sync.Pool{
		New: func() any {
			return util.NewBuffer(maxFrameSize)
		},
	}
)

func msg(vals ...any) *util.Buffer {
	buf := bufPool.Get().(*util.Buffer)

	for _, val := range vals {
		buf.Put(val)
	}

	return buf
}

func (s *session) readAndExec(r io.Reader, writeCh chan<- *util.Buffer) (err error) {
	var clientMark uint8
	if err = binary.Read(r, binary.LittleEndian, &clientMark); err != nil {
		return
	}

	var cmd uint8
	if err = binary.Read(r, binary.LittleEndian, &cmd); err != nil {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "Bad command format")
		return
	}

	//log.Debug().Uint8("Cm", clientMark).Uint8("Op", cmd).Msg("Recived commnad")

	s.wg.Add(1)
	return s.doCommandCall(clientMark, cmd, r, writeCh)
}

func osErrCode(err error) uint8 {
	if os.IsExist(err) {
		return wsfsprotocol.ErrorExists
	} else if os.IsNotExist(err) {
		return wsfsprotocol.ErrorNotExists
	} else if os.IsPermission(err) {
		return wsfsprotocol.ErrorAccess
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

// base ends with "/"
func (s *session) sendDirent(clientMark uint8, writeCh chan<- *util.Buffer, base string, dirents []fs.DirEntry, okCode uint8) {
	rsp := bufPool.Get().(*util.Buffer)
	rsp.Put(clientMark)
	rsp.Put(okCode)
	for _, dirent := range dirents {
		rsp.Put(dirent.Name())
		fi, err := dirent.Info()
		if err == nil && fi.Mode()&fs.ModeSymlink != 0 {
			fi, err = s.restrictingSymlinkByFileInfo(base, fi)
		}
		if err != nil {
			//log.Error().Err(err).Str("File", dirent.Name()).Str("Dir", base).Msg("get file info failed")
			rsp.Put(uint64(0))
			rsp.Put(int64(0))
			rsp.Put(uint32(os.ModeIrregular))
			rsp.Put(wsfsprotocol.OWNER_NN)
		} else {
			rsp.Put(uint64(fi.Size()))
			rsp.Put(fi.ModTime().Unix())
			rsp.Put(uint32(fi.Mode()))
			rsp.Put(s.convOwner(fi))
		}
	}
	writeCh <- rsp
}

func (s *session) cmdReadDir(clientMark uint8, writeCh chan<- *util.Buffer, path string) {
	defer s.wg.Done()

	if !util.IsUrlValid(path) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + path

	f, err := os.Open(apath)
	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "open dir failed")
		return
	}
	defer func() {
		if f.Close() != nil {
			log.Error().Err(err).Str("Path", apath).Msg("close dir failed")
		}
	}()

	dirents, err := f.ReadDir(-1)
	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "read dir failed")
		return
	}

	if len(dirents) == 0 {
		s.sendDirent(clientMark, writeCh, apath, dirents, wsfsprotocol.ErrorOK)
		return
	}

	lastIndex := 0
	msgSize := 2
	for i, dirent := range dirents {
		entSize := 0
		entSize += len(dirent.Name()) + 1
		entSize += 21
		if msgSize+entSize > maxFrameSize {
			if i == len(dirents)-1 {
				s.sendDirent(clientMark, writeCh, apath, dirents[lastIndex:i], wsfsprotocol.ErrorOK)
			} else {
				s.sendDirent(clientMark, writeCh, apath, dirents[lastIndex:i], wsfsprotocol.ErrorPartialResponse)
			}
			lastIndex = i
			msgSize = 2
		} else {
			msgSize += entSize
		}
	}
	if msgSize != 2 {
		s.sendDirent(clientMark, writeCh, apath, dirents[lastIndex:], wsfsprotocol.ErrorOK)
	}
}

func (s *session) cmdGetAttr(clientMark uint8, writeCh chan<- *util.Buffer, lpath string) {
	defer s.wg.Done()

	if !util.IsUrlValid(lpath) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + lpath

	rsp := bufPool.Get().(*util.Buffer)
	fi, err := os.Stat(apath)
	if err != nil {
		goto BAD
	}

	rsp.Put(clientMark)
	rsp.Put(wsfsprotocol.ErrorOK)
	if fi.Mode()&fs.ModeSymlink != 0 {
		fi, err = s.restrictingSymlinkByFileInfo(apath, fi)
	}
	if err != nil {
		goto BAD
	}
	rsp.Put(uint64(fi.Size()))
	rsp.Put(fi.ModTime().Unix())
	rsp.Put(uint32(fi.Mode()))
	rsp.Put(s.convOwner(fi))
	writeCh <- rsp
	return
BAD:
	rsp.Done()
	rsp.Put(clientMark)
	rsp.Put(osErrCode(err))
	rsp.Put("syscall error")
	writeCh <- rsp
}

func (s *session) treeADir(depth uint8, path string, hint string,
	fillEntry func(base string, entry fs.DirEntry, hint string),
	fillStatus func(status uint8)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}

	dirents, err := f.ReadDir(-1)
	if err != nil {
		f.Close()
		return
	}

	fillStatus(wsfsprotocol.TREEDIR_STATUS_ENTER_DIR)
	if len(dirents) == 0 {
		fillStatus(wsfsprotocol.TREEDIR_STATUS_END_DIR)
		return
	}

	for _, dirent := range dirents {
		fillEntry(path, dirent, hint)
		if dirent.IsDir() && depth > 0 {
			s.treeADir(depth-1, path+dirent.Name()+"/", hint, fillEntry, fillStatus)
		}
	}
	fillStatus(wsfsprotocol.TREEDIR_STATUS_END_DIR)

	if f.Close() != nil {
		log.Error().Err(err).Str("Path", path).Msg("close dir failed")
	}
}

func (s *session) cmdTreeDir(clientMark uint8, writeCh chan<- *util.Buffer, path string, depth uint8, hint string) {
	defer s.wg.Done()

	if !util.IsUrlValid(path) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + path

	rsp := bufPool.Get().(*util.Buffer)
	rsp.Put(clientMark)
	rsp.Put(wsfsprotocol.ErrorPartialResponse)
	sendRsp := func() {
		writeCh <- rsp
		rsp = bufPool.Get().(*util.Buffer)
		rsp.Put(clientMark)
		rsp.Put(wsfsprotocol.ErrorPartialResponse)
	}

	s.treeADir(depth, apath, hint,
		func(base string, entry fs.DirEntry, hint string) {
			fi, err := entry.Info()
			if err == nil && fi.Mode()&fs.ModeSymlink != 0 {
				fi, err = s.restrictingSymlinkByFileInfo(base, fi)
			}

			var fileData []byte = nil
			if err == nil &&
				fi.Mode()&fs.ModeSymlink == 0 &&
				entry.Name() == hint &&
				2+1+int64(len(entry.Name()))+1+21+fi.Size() <= int64(maxFrameSize) {
				fileData, err = os.ReadFile(base + entry.Name())
				if err != nil {
					fileData = nil
					err = nil
				}
			}

			if fileData != nil {
				if rsp.Len()+1+len(entry.Name())+1+21+len(fileData) > maxFrameSize {
					sendRsp()
				}
			} else if rsp.Len()+1+len(entry.Name())+1+21 > maxFrameSize {
				sendRsp()
			}

			if fileData != nil {
				rsp.Put(wsfsprotocol.TREEDIR_STATUS_OK_WITH_DATA)
			} else {
				rsp.Put(wsfsprotocol.TREEDIR_STATUS_OK)
			}

			rsp.Put(entry.Name())
			if err != nil {
				//log.Error().Err(err).Str("File", dirent.Name()).Str("Dir", base).Msg("get file info failed")
				rsp.Put(uint64(0))
				rsp.Put(int64(0))
				rsp.Put(uint32(os.ModeIrregular))
				rsp.Put(wsfsprotocol.OWNER_NN)
			} else {
				rsp.Put(uint64(fi.Size()))
				rsp.Put(fi.ModTime().Unix())
				rsp.Put(uint32(fi.Mode()))
				rsp.Put(s.convOwner(fi))
			}
			if fileData != nil {
				rsp.Put(fileData)
			}
		}, func(status uint8) {
			if rsp.Len()+1 > maxFrameSize {
				sendRsp()
			}
			rsp.Put(status)
		})

	if rsp.Len() != 2 {
		rsp.ModifyByteAt(1, wsfsprotocol.ErrorOK)
		writeCh <- rsp
	} else {
		rsp.Done()
		bufPool.Put(rsp)
	}
}
