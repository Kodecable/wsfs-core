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
			rsp.Put(fi.ModTime().Unix())
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
