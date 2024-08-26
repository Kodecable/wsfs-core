package wsfs

import (
	"encoding/binary"
	"errors"
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
	dataPerMsg     uint64 = 4096
	maxDataInARead uint64 = dataPerMsg * 16
)

var (
	bufPool = sync.Pool{
		New: func() any {
			return util.NewBuffer(int(dataPerMsg) + 2)
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

func (s *session) readAndExec(r io.Reader, writeCh chan<- *util.Buffer) error {
	var err error
	var clientMark uint8
	if err = binary.Read(r, binary.LittleEndian, &clientMark); err != nil {
		return err
	}

	var cmd uint8
	if err = binary.Read(r, binary.LittleEndian, &cmd); err != nil {
		goto BadCmd
	}

	//log.Debug().Uint8("Cm", clientMark).Uint8("Op", cmd).Msg("Recived commnad")

	switch cmd {
	// CODE BELOW GENARATED BY genCommandCalls.py
	case wsfsprotocol.CmdOpen:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint32
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		var v2 uint32
		err = binary.Read(r, binary.LittleEndian, &v2)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdOpen(clientMark, writeCh, v0, v1, v2)
	case wsfsprotocol.CmdClose:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdClose(clientMark, writeCh, v0)
	case wsfsprotocol.CmdRead:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint64
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdRead(clientMark, writeCh, v0, v1)
	case wsfsprotocol.CmdReadDir:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdReadDir(clientMark, writeCh, v0)
	case wsfsprotocol.CmdReadLink:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdReadLink(clientMark, writeCh, v0)
	case wsfsprotocol.CmdWrite:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 = bufPool.Get().(*util.Buffer)
		_, err = io.Copy(v1, r)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdWrite(clientMark, writeCh, v0, v1)
	case wsfsprotocol.CmdSeek:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint8
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		var v2 int64
		err = binary.Read(r, binary.LittleEndian, &v2)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdSeek(clientMark, writeCh, v0, v1, v2)
	case wsfsprotocol.CmdAllocate:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint32
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		var v2 uint64
		err = binary.Read(r, binary.LittleEndian, &v2)
		if err != nil {
			goto BadCmd
		}
		var v3 uint64
		err = binary.Read(r, binary.LittleEndian, &v3)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdAllocate(clientMark, writeCh, v0, v1, v2, v3)
	case wsfsprotocol.CmdGetAttr:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdGetAttr(clientMark, writeCh, v0)
	case wsfsprotocol.CmdSetAttr:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint8
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		var v2 uint64
		err = binary.Read(r, binary.LittleEndian, &v2)
		if err != nil {
			goto BadCmd
		}
		var v3 int64
		err = binary.Read(r, binary.LittleEndian, &v3)
		if err != nil {
			goto BadCmd
		}
		var v4 uint32
		err = binary.Read(r, binary.LittleEndian, &v4)
		if err != nil {
			goto BadCmd
		}
		var v5 uint8
		err = binary.Read(r, binary.LittleEndian, &v5)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdSetAttr(clientMark, writeCh, v0, v1, v2, v3, v4, v5)
	case wsfsprotocol.CmdSync:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdSync(clientMark, writeCh, v0)
	case wsfsprotocol.CmdMkdir:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint32
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdMkdir(clientMark, writeCh, v0, v1)
	case wsfsprotocol.CmdSymLink:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 string
		err = util.CopyStrFromReader(r, &v1)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdSymLink(clientMark, writeCh, v0, v1)
	case wsfsprotocol.CmdRemove:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdRemove(clientMark, writeCh, v0)
	case wsfsprotocol.CmdRmDir:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdRmDir(clientMark, writeCh, v0)
	case wsfsprotocol.CmdFsStat:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdFsStat(clientMark, writeCh, v0)
	case wsfsprotocol.CmdReadAt:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint64
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		var v2 uint64
		err = binary.Read(r, binary.LittleEndian, &v2)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdReadAt(clientMark, writeCh, v0, v1, v2)
	case wsfsprotocol.CmdWriteAt:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint64
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		var v2 = bufPool.Get().(*util.Buffer)
		_, err = io.Copy(v2, r)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdWriteAt(clientMark, writeCh, v0, v1, v2)
	case wsfsprotocol.CmdCopyFileRange:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint32
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		var v2 uint64
		err = binary.Read(r, binary.LittleEndian, &v2)
		if err != nil {
			goto BadCmd
		}
		var v3 uint64
		err = binary.Read(r, binary.LittleEndian, &v3)
		if err != nil {
			goto BadCmd
		}
		var v4 uint64
		err = binary.Read(r, binary.LittleEndian, &v4)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdCopyFileRange(clientMark, writeCh, v0, v1, v2, v3, v4)
	case wsfsprotocol.CmdRename:
		var v0 string
		err = util.CopyStrFromReader(r, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 string
		err = util.CopyStrFromReader(r, &v1)
		if err != nil {
			goto BadCmd
		}
		var v2 uint32
		err = binary.Read(r, binary.LittleEndian, &v2)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdRename(clientMark, writeCh, v0, v1, v2)
	case wsfsprotocol.CmdSetAttrByFD:
		var v0 uint32
		err = binary.Read(r, binary.LittleEndian, &v0)
		if err != nil {
			goto BadCmd
		}
		var v1 uint8
		err = binary.Read(r, binary.LittleEndian, &v1)
		if err != nil {
			goto BadCmd
		}
		var v2 uint64
		err = binary.Read(r, binary.LittleEndian, &v2)
		if err != nil {
			goto BadCmd
		}
		var v3 int64
		err = binary.Read(r, binary.LittleEndian, &v3)
		if err != nil {
			goto BadCmd
		}
		var v4 uint32
		err = binary.Read(r, binary.LittleEndian, &v4)
		if err != nil {
			goto BadCmd
		}
		var v5 uint8
		err = binary.Read(r, binary.LittleEndian, &v5)
		if err != nil {
			goto BadCmd
		}
		s.wg.Add(1)
		s.cmdSetAttrByFD(clientMark, writeCh, v0, v1, v2, v3, v4, v5)
	// CODE ABOVE GENARATED BY genCommandCalls.py
	default:
		err = errors.New("unknwon CommandCode")
		goto BadCmd
	}
	return nil

BadCmd:
	writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "Bad command format or unknown command")
	return err
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

func (s *session) restrictingSymlinkByFileInfo(base string, fi fs.FileInfo) (fs.FileInfo, error) {
	target, err := os.Readlink(path.Join(base, fi.Name()))
	if err != nil {
		return fi, err
	}

	if target[0] != '/' {
		target = path.Clean(path.Join(base, target))
	}

	if strings.HasPrefix(target, s.storage.Path) {
		return fi, nil
	} else {
		return os.Stat(target)
	}
}

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
			rsp.Put((fi.ModTime().Unix()))
			rsp.Put(uint32(fi.Mode()))
			rsp.Put(s.convOwner(fi))
		}
	}
	writeCh <- rsp
}

func (s *session) cmdReadDir(clientMark uint8, writeCh chan<- *util.Buffer, path string) {
	defer s.wg.Done()
	const maxDrentInARsp = 16

	if !util.IsUrlValid(path) {
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "bad path")
		return
	}
	apath := s.storage.Path + path

	f, err := os.Open(apath)
	if err != nil {
		writeCh <- msg(clientMark, osErrCode(err), "open dir failed")
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

	if len(dirents) < maxDrentInARsp {
		s.sendDirent(clientMark, writeCh, path, dirents, wsfsprotocol.ErrorOK)
	} else {
		var off int = 0
		for range len(dirents) / maxDrentInARsp {
			s.sendDirent(clientMark, writeCh, path, dirents[off:off+maxDrentInARsp], wsfsprotocol.ErrorPartialResponse)
			off += maxDrentInARsp
		}
		s.sendDirent(clientMark, writeCh, path, dirents[off:], wsfsprotocol.ErrorOK)
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
	rsp.Put((fi.ModTime().Unix()))
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
