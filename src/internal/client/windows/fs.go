//go:build windows

package windows

import (
	"os"
	"path/filepath"
	"strings"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/winfsp/cgofuse/fuse"
)

const (
	ok              = 0
	fsBlockSize     = 4096
	fsFileNameLen   = 255
	defaultFileMode = 0o644
)

func fileMode(mode uint32) (r uint32) {
	r = 0o777

	if mode&uint32(os.ModeSetuid) != 0 {
		r |= fuse.S_ISUID
	}
	if mode&uint32(os.ModeSetgid) != 0 {
		r |= fuse.S_ISGID
	}
	if mode&uint32(os.ModeSticky) != 0 {
		r |= fuse.S_ISVTX
	}

	if mode&uint32(os.ModeDir) != 0 {
		r |= fuse.S_IFDIR
	} else if mode&uint32(os.ModeSymlink) != 0 {
		r |= fuse.S_IFLNK
	} else if mode&(uint32(os.ModeDevice)|
		uint32(os.ModeNamedPipe)|
		uint32(os.ModeSocket)) != 0 {
		return r & ^uint32(0o777)
	} else {
		r |= fuse.S_IFREG
	}

	return
}

func statFromFileInfo(stat *fuse.Stat_t, fi *session.FileInfo) {
	stat.Size = int64(fi.Size)
	stat.Atim = fuse.Timespec{Sec: fi.MTime, Nsec: 0}
	stat.Ctim = fuse.Timespec{Sec: fi.MTime, Nsec: 0}
	stat.Mtim = fuse.Timespec{Sec: fi.MTime, Nsec: 0}
	stat.Mode = fileMode(fi.Mode)
	stat.Nlink = 1
	stat.Blksize = fsBlockSize

	stat.Blocks = int64(fi.Size / 512)
	if fi.Size%512 != 0 {
		stat.Blocks += 1
	}

	// We set the 'Uid=-1,Gid=-1' flag to ignore file owner field,
	// so there is no need to set file's uid and gid here
}

func statFromDirItem(stat *fuse.Stat_t, di *session.DirItem) {
	statFromFileInfo(stat, &session.FileInfo{
		Size:  di.Size,
		MTime: di.MTime,
		Mode:  di.Mode,
		Owner: di.Owner,
	})
}

type fileSystem struct {
	fuse.FileSystemBase

	session    *session.Session
	mountpoint string
}

func NewFS(session *session.Session, mountpoint string) *fileSystem {
	return &fileSystem{
		session:    session,
		mountpoint: mountpoint,
	}
}

func (s *fileSystem) Statfs(path string, stat *fuse.Statfs_t) int {
	fsi, code := s.session.CmdFsStat(path)
	if code != wsfsprotocol.ErrorOK {
		return errorCodeMap[code]
	}

	stat.Frsize = fsBlockSize
	stat.Bsize = fsBlockSize
	stat.Blocks = fsi.Total / fsBlockSize
	stat.Bfree = fsi.Free / fsBlockSize
	stat.Bavail = fsi.Available / fsBlockSize
	stat.Namemax = fsFileNameLen
	stat.Files = 0
	stat.Ffree = 0
	return ok
}

func (s *fileSystem) Open(path string, flags int) (errc int, fh uint64) {
	fd, code := s.session.CmdOpen(path, uint32(flags), defaultFileMode)
	return errorCodeMap[code], uint64(fd)
}

func (s *fileSystem) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	fi, code := s.session.CmdGetAttr(path)
	if code != wsfsprotocol.ErrorOK {
		return errorCodeMap[code]
	}

	statFromFileInfo(stat, &fi)
	return ok
}

func (s *fileSystem) Mkdir(path string, mode uint32) int {
	code := s.session.CmdMkdir(path, mode)
	return errorCodeMap[code]
}

func (s *fileSystem) Unlink(path string) int {
	code := s.session.CmdRemove(path)
	return errorCodeMap[code]
}

func (s *fileSystem) Rmdir(path string) int {
	code := s.session.CmdRmDir(path)
	return errorCodeMap[code]
}

func (s *fileSystem) Symlink(target string, newpath string) int {
	if !strings.HasPrefix(target, s.mountpoint) {
		return -fuse.EACCES
	}
	target = strings.TrimPrefix(target, s.mountpoint)

	code := s.session.CmdSymLink(target, newpath)
	return errorCodeMap[code]
}

func (s *fileSystem) Readlink(path string) (int, string) {
	path, code := s.session.CmdReadLink(path)
	if code != wsfsprotocol.ErrorOK {
		return errorCodeMap[code], ""
	}
	return ok, filepath.Join(s.mountpoint, path)
}

func (s *fileSystem) Rename(oldpath string, newpath string) int {
	code := s.session.CmdRename(oldpath, newpath, 0)
	return errorCodeMap[code]
}

/*
func (s *fs) Access(path string, mask uint32) int {

		mask = mask & 7
		if mask == 0 {
			return ok
		}

		fi, code := s.session.CmdGetAttr(path)
		if code != wsfsprotocol.ErrorOK {
			return errorCodeMap[code]
		}
		mode := fi.Mode & 0o777

		if fi.Owner&0b01 != 0 {
			if mode&(mask<<6) != 0 {
				return ok
			}
		}
		if fi.Owner&0b10 != 0 {
			if mode&(mask<<3) != 0 {
				return ok
			}
		}
		if mode&mask != 0 {
			return ok
		}
		return -fuse.EACCES
}*/

func (s *fileSystem) Create(path string, flags int, mode uint32) (int, uint64) {
	fd, code := s.session.CmdOpen(path, uint32(flags|os.O_CREATE), mode)
	return errorCodeMap[code], uint64(fd)
}

func (s *fileSystem) Truncate(path string, size int64, fh uint64) int {
	var code uint8
	if ^uint64(0) == fh {
		code = s.session.CmdSetAttr(path,
			wsfsprotocol.SETATTR_SIZE,
			session.FileInfo{Size: uint64(size)})
	} else {
		code = s.session.CmdSetAttrByFD(uint32(fh),
			wsfsprotocol.SETATTR_SIZE,
			session.FileInfo{Size: uint64(size)})
	}
	return errorCodeMap[code]
}

func (s *fileSystem) Read(path string, buff []byte, ofst int64, fh uint64) int {
	readed, code := s.session.CmdReadAt(uint32(fh), uint64(ofst), buff)
	if code != wsfsprotocol.ErrorOK {
		return errorCodeMap[code]
	}
	return int(readed)
}

func (s *fileSystem) Write(path string, buff []byte, ofst int64, fh uint64) int {
	count, code := s.session.CmdWriteAt(uint32(fh), uint64(ofst), buff)
	if code != wsfsprotocol.ErrorOK {
		return errorCodeMap[code]
	}
	return int(count)
}

func (*fileSystem) Flush(_ string, _ uint64) int {
	return ok
}

func (s *fileSystem) Release(_ string, fh uint64) int {
	code := s.session.CmdClose(uint32(fh))
	return errorCodeMap[code]
}

func (s *fileSystem) Fsync(_ string, _ bool, fh uint64) int {
	code := s.session.CmdSync(uint32(fh))
	return errorCodeMap[code]
}

func (s *fileSystem) Opendir(path string) (int, uint64) {
	const O_DIRECTORY = 0x10000
	fd, code := s.session.CmdOpen(path, uint32(os.O_RDONLY|O_DIRECTORY), 0)
	return errorCodeMap[code], uint64(fd)
}

func (s *fileSystem) Releasedir(_ string, fh uint64) int {
	code := s.session.CmdClose(uint32(fh))
	return errorCodeMap[code]
}

func (s *fileSystem) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, _ int64, _ uint64) int {
	items, code := s.session.CmdReadDir(path)
	if code != wsfsprotocol.ErrorOK {
		return errorCodeMap[code]
	}

	fill(".", nil, 0)
	fill("..", nil, 0)
	for _, item := range items {
		stat := fuse.Stat_t{}
		statFromDirItem(&stat, &item)
		fill(item.Name, &stat, 0)
	}

	return ok
}
