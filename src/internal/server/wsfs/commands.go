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

func (s *session) dispatchCommand(r io.Reader) (err error) {
	var header [2]byte
	_, err = io.ReadFull(r, header[:])
	if err != nil {
		return fmt.Errorf("bad command header: %e", err)
	}
	log.Debug().Uint8("Cm", header[0]).Uint8("Op", header[1]).Msg("Recived commnad")
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
	defer bufPool.Put(rsp)
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

func (s *session) treeADir(depth uint8, path string, hint string,
	writeEntry func(base string, entry fs.DirEntry, hint string),
	writeIndicator func(status uint8)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}

	dirents, err := f.ReadDir(-1)
	if err != nil {
		f.Close()
		return
	}

	writeIndicator(wsfsprotocol.TREEDIR_INDICATOR_ENTER_DIR)
	if len(dirents) == 0 {
		writeIndicator(wsfsprotocol.TREEDIR_INDICATOR_END_DIR)
		return
	}

	for _, dirent := range dirents {
		writeEntry(path, dirent, hint)
		if dirent.IsDir() && depth > 0 {
			s.treeADir(depth-1, path+dirent.Name()+"/", hint, writeEntry, writeIndicator)
		}
	}
	writeIndicator(wsfsprotocol.TREEDIR_INDICATOR_END_DIR)

	if f.Close() != nil {
		log.Error().Err(err).Str("Path", path).Msg("close dir failed")
	}
}

func (s *session) cmdTreeDir(clientMark uint8, req wsfsprotocol.CmdTreeDirStruct) {
	path, depth, hint := req.Path, req.Depth, req.Hint

	if !util.IsUrlValid(path) {
		s.writeRspError(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + path

	rsp := bufPool.Get().(*util.Buffer)
	rsp.Write([]byte{clientMark, wsfsprotocol.ErrorPartialResponse})
	sendRsp := func() {
		s.write(rsp.Done())
		bufPool.Put(rsp)
		rsp = bufPool.Get().(*util.Buffer)
		rsp.Write([]byte{clientMark, wsfsprotocol.ErrorPartialResponse})
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
				int64(2+1+wsfsprotocol.GetDirentRequiredSize(wsfsprotocol.Dirent{Name: entry.Name()}))+fi.Size() <= int64(MaxMsgSize) {
				fileData, err = os.ReadFile(base + entry.Name())
				if err != nil {
					fileData = nil
					err = nil
				}
			}

			n := wsfsprotocol.GetDirentRequiredSize(wsfsprotocol.Dirent{Name: entry.Name()})
			if fileData != nil {
				if rsp.Writted()+1+n+len(fileData) > MaxMsgSize {
					sendRsp()
				}
			} else if rsp.Writted()+1+n > MaxMsgSize {
				sendRsp()
			}

			if fileData != nil {
				rsp.Write([]byte{wsfsprotocol.TREEDIR_INDICATOR_FILE_WITH_DATA})
			} else {
				rsp.Write([]byte{wsfsprotocol.TREEDIR_INDICATOR_FILE})
			}

			rsp.Write([]byte(entry.Name()))
			rsp.Write([]byte{0})
			if err != nil {
				//log.Error().Err(err).Str("File", dirent.Name()).Str("Dir", base).Msg("get file info failed")
				wsfsprotocol.WriteFileInfoToWriter(wsfsprotocol.FileInfo{Mode: uint32(os.ModeIrregular), Owner: wsfsprotocol.OWNER_NN}, rsp)
			} else {
				wsfsprotocol.WriteFileInfoToWriter(wsfsprotocol.FileInfo{Size: uint64(fi.Size()), MTime: fi.ModTime().Unix(), Mode: uint32(fi.Mode()), Owner: s.convOwner(fi)}, rsp)
			}
			if fileData != nil {
				rsp.Write(fileData)
			}
		}, func(status uint8) {
			if rsp.Writted()+1 > MaxMsgSize {
				sendRsp()
			}
			rsp.Write([]byte{status})
		})

	if rsp.Writted() != 2 {
		rsp.Bytes[1] = wsfsprotocol.ErrorOK
		s.write(rsp.Done())
	} else {
		rsp.Done()
	}
	bufPool.Put(rsp)
}
