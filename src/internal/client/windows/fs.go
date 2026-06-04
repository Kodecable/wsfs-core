//go:build windows

package windows

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/winfsp/cgofuse/fuse"
)

const (
	fuseOK          = 0
	fsBlockSize     = 4096
	fsFileNameLen   = 255
	defaultFileMode = 0o644
	defaultDirMode  = 0o755
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
		return fuse.S_IFREG // with 000 permission
	} else {
		r |= fuse.S_IFREG
	}

	return
}

func statFromFileInfo(stat *fuse.Stat_t, fi *wsfsprotocol.FileInfo) {
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
	statFromFileInfo(stat, &wsfsprotocol.FileInfo{
		Size:  di.Size,
		MTime: di.MTime,
		Mode:  di.Mode,
		Owner: di.Owner,
	})
}

func fileInfoFromDirItem(di *session.DirItem) wsfsprotocol.FileInfo {
	return wsfsprotocol.FileInfo{
		Size:  di.Size,
		MTime: di.MTime,
		Mode:  di.Mode,
		Owner: di.Owner,
	}
}

func (s *fileSystem) cachePrefetchedDirChildren(parentPath string, items []session.DirItem) {
	go func() {
		for i := range items {
			item := &items[i]
			pendingCh := item.PrefetchReady()
			if pendingCh == nil {
				continue
			}
			childPath := parentPath + item.Name + "/"
			pendingKey := dirCacheKey(childPath)
			item.WaitPrefetch()
			if item.Child == nil {
				s.deletePendingIfMatch(pendingKey, pendingCh)
				continue
			}
			s.cache.Set(dirCacheKey(childPath), cachedData{
				attr:  fileInfoFromDirItem(item),
				items: item.Child,
			})
			s.deletePendingIfMatch(pendingKey, pendingCh)
		}
	}()
}

// O_DIRECTORY is missing because win has no equivalent const
func wsfsOpenFlagFromWinOpenFlag(winflag int) (flag uint32) {
	//if winflag&fuse.O_RDONLY != 0 {
	//	flag |= wsfsprotocol.O_RDONLY
	//}
	if winflag&fuse.O_WRONLY != 0 {
		flag |= wsfsprotocol.O_WRONLY
	}
	if winflag&fuse.O_RDWR != 0 {
		flag |= wsfsprotocol.O_RDWR
	}
	if winflag&fuse.O_TRUNC != 0 {
		flag |= wsfsprotocol.O_TRUNC
	}
	if winflag&fuse.O_EXCL != 0 {
		flag |= wsfsprotocol.O_EXCL
	}
	if winflag&fuse.O_CREAT != 0 {
		flag |= wsfsprotocol.O_CREAT
	}
	if winflag&fuse.O_APPEND != 0 {
		flag |= wsfsprotocol.O_APPEND
	}
	return
}

type fileSystem struct {
	fuse.FileSystemBase

	cache      fsCache
	pending    sync.Map
	session    *session.Session
	mountpoint string
	onDestroy  func()
}

func NewFS(session *session.Session, mountpoint string, timeout time.Duration, onDestroy func()) *fileSystem {
	s := &fileSystem{
		session:    session,
		mountpoint: mountpoint,
		onDestroy:  onDestroy,
		cache: fsCache{
			timeout: timeout,
		},
	}
	go s.cache.Run()
	return s
}

func normalizeCachePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	return path
}

func pathCacheKey(path string) string {
	return normalizeCachePath(path)
}

func dirCacheKey(path string) string {
	path = normalizeCachePath(path)
	if path == "/" {
		return "/"
	}
	return path + "/"
}

func (s *fileSystem) delParentCache(path string) {
	if ppath := filepath.Dir(path); ppath != path {
		s.deleteDirState(dirCacheKey(ppath))
	}
}

func (s *fileSystem) deleteDirState(key string) {
	s.cache.Del(key)
	s.pending.Delete(key)
}

func (s *fileSystem) deletePathState(path string) {
	s.cache.Del(pathCacheKey(path))
	s.deleteDirState(dirCacheKey(path))
}

func (s *fileSystem) loadPendingDir(key string) (<-chan struct{}, bool) {
	v, ok := s.pending.Load(key)
	if !ok {
		return nil, false
	}
	return v.(<-chan struct{}), true
}

func (s *fileSystem) lookupChildFromDirCache(dirKey, name string, stat *fuse.Stat_t) (int, bool) {
	cache, ok := s.cache.Get(dirKey)
	if !ok || cache.items == nil {
		return 0, false
	}
	for _, item := range cache.items {
		if item.Name == name {
			statFromDirItem(stat, &item)
			return fuseOK, true
		}
	}
	return -fuse.ENOENT, true
}

func (s *fileSystem) storePendingDir(key string, ready <-chan struct{}) {
	s.pending.Store(key, ready)
}

func (s *fileSystem) deletePendingIfMatch(key string, ready <-chan struct{}) {
	v, ok := s.pending.Load(key)
	if ok && v.(<-chan struct{}) == ready {
		s.pending.Delete(key)
	}
}

func (s *fileSystem) Statfs(path string, stat *fuse.Statfs_t) int {
	fsi, code := s.session.CmdFsStat(path)
	if code != wsfsprotocol.ErrorOK {
		return errnoFromCode(code)
	}

	stat.Frsize = fsBlockSize
	stat.Bsize = fsBlockSize
	stat.Blocks = fsi.Total / fsBlockSize
	stat.Bfree = fsi.Free / fsBlockSize
	stat.Bavail = fsi.Available / fsBlockSize
	stat.Namemax = fsFileNameLen
	stat.Files = 0
	stat.Ffree = 0
	return fuseOK
}

func (s *fileSystem) Open(path string, winflag int) (errc int, fh uint64) {
	if winflag&fuse.O_WRONLY != 0 || winflag&fuse.O_RDWR != 0 {
		s.delParentCache(path)
		s.deletePathState(path)
	}

	fd, code := s.session.CmdOpen(path, wsfsOpenFlagFromWinOpenFlag(winflag), defaultFileMode)
	//log.Warn().Str("Path", path).Int("Flag", winflag).Uint32("Fd", fd).Msg("Open")
	return errnoFromCode(code), uint64(fd)
}

func (s *fileSystem) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	if ppath := filepath.Dir(path); ppath != path {
		dirKey := dirCacheKey(ppath)
		name := filepath.Base(path)
		if errc, ok := s.lookupChildFromDirCache(dirKey, name, stat); ok {
			return errc
		}
		if pendingCh, ok := s.loadPendingDir(dirKey); ok {
			<-pendingCh
			if errc, ok := s.lookupChildFromDirCache(dirKey, name, stat); ok {
				return errc
			}
			s.pending.Delete(dirKey)
		}
	}

	if cache, ok := s.cache.Get(pathCacheKey(path)); ok {
		statFromFileInfo(stat, &cache.attr)
		return fuseOK
	}

	//log.Warn().Str("Path", path).Uint64("fh", fh).Msg("Getattr")
	fi, code := s.session.CmdGetAttr(path)
	if code != wsfsprotocol.ErrorOK {
		return errnoFromCode(code)
	}
	s.cache.Set(pathCacheKey(path), cachedData{attr: fi})

	statFromFileInfo(stat, &fi)
	return fuseOK
}

func (s *fileSystem) Mkdir(path string, mode uint32) int {
	//log.Warn().Str("Path", path).Uint32("mode", mode).Msg("Mkdir")
	s.delParentCache(path)
	code := s.session.CmdMkdir(path, defaultDirMode)
	return errnoFromCode(code)
}

func (s *fileSystem) Unlink(path string) int {
	//log.Warn().Str("Path", path).Msg("Unlink")
	s.delParentCache(path)
	s.deletePathState(path)
	code := s.session.CmdRemove(path)
	return errnoFromCode(code)
}

func (s *fileSystem) Rmdir(path string) int {
	//log.Warn().Str("Path", path).Msg("Rmdir")
	s.delParentCache(path)
	s.deletePathState(path)
	code := s.session.CmdRmDir(path)
	return errnoFromCode(code)
}

func (s *fileSystem) Symlink(target string, newpath string) int {
	//log.Warn().Str("Target", target).Str("Path", newpath).Msg("Symlink")
	if !strings.HasPrefix(target, s.mountpoint) {
		return -fuse.EACCES
	}
	target = strings.TrimPrefix(target, s.mountpoint)

	s.delParentCache(target)
	s.deletePathState(target)
	s.delParentCache(newpath)
	s.deletePathState(newpath)

	code := s.session.CmdSymLink(target, newpath)
	return errnoFromCode(code)
}

func (s *fileSystem) Readlink(path string) (int, string) {
	//log.Warn().Str("Path", path).Msg("Readlink")
	path, code := s.session.CmdReadLink(path)
	if code != wsfsprotocol.ErrorOK {
		return errnoFromCode(code), ""
	}
	return fuseOK, filepath.Join(s.mountpoint, path)
}

func (s *fileSystem) Rename(oldpath string, newpath string) int {
	//log.Warn().Str("OldPath", oldpath).Str("NewPath", newpath).Msg("Rename")

	s.delParentCache(oldpath)
	s.deletePathState(oldpath)
	s.delParentCache(newpath)
	s.deletePathState(newpath)

	code := s.session.CmdRename(oldpath, newpath, 0)
	return errnoFromCode(code)
}

/*
func (s *fs) Access(path string, mask uint32) int {

		mask = mask & 7
		if mask == 0 {
			return ok
		}

		fi, code := s.session.CmdGetAttr(path)
		if code != wsfsprotocol.ErrorOK {
			return errnoFromCode(code)
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
	s.delParentCache(path)
	s.deletePathState(path)

	fd, code := s.session.CmdOpen(path, wsfsOpenFlagFromWinOpenFlag(flags), defaultFileMode)
	//log.Warn().Str("Path", path).Int("Flags", flags).Uint32("Mode", mode).Uint32("Fd", fd).Msg("Create")
	return errnoFromCode(code), uint64(fd)
}

func (s *fileSystem) Truncate(path string, size int64, fh uint64) int {
	s.delParentCache(path)
	s.deletePathState(path)

	//log.Error().Str("Path", path).Int64("Size", size).Uint64("Fd", fh).Msg("Truncate")
	var code uint8
	if ^uint64(0) == fh {
		code = s.session.CmdSetAttr(path,
			wsfsprotocol.SETATTR_SIZE,
			wsfsprotocol.FileInfo{Size: uint64(size)})
	} else {
		code = s.session.CmdSetAttrByFD(uint32(fh),
			wsfsprotocol.SETATTR_SIZE,
			wsfsprotocol.FileInfo{Size: uint64(size)})
	}
	return errnoFromCode(code)
}

func (s *fileSystem) Read(path string, buff []byte, ofst int64, fh uint64) int {
	readed, code := s.session.CmdReadAt(uint32(fh), uint64(ofst), buff)
	//log.Error().Str("Path", path).Int("Want", len(buff)).Int64("Ofst", ofst).Uint64("Readed", readed).Uint64("Fd", fh).Msg("Read")
	if code != wsfsprotocol.ErrorOK {
		return errnoFromCode(code)
	}
	return int(readed)
}

func (s *fileSystem) Write(path string, buff []byte, ofst int64, fh uint64) int {
	count, code := s.session.CmdWriteAt(uint32(fh), uint64(ofst), buff)
	if code != wsfsprotocol.ErrorOK {
		return errnoFromCode(code)
	}
	//log.Error().Str("Path", path).Int64("ofst", ofst).Int("Want", len(buff)).Uint64("Writed", count).Uint64("Fd", fh).Msg("Write")
	return int(count)
}

func (*fileSystem) Flush(_ string, _ uint64) int {
	return fuseOK
}

func (s *fileSystem) Release(path string, fh uint64) int {
	s.delParentCache(path)
	s.deletePathState(path)

	//log.Warn().Uint64("Fd", fh).Msg("Release")
	code := s.session.CmdClose(uint32(fh))
	return errnoFromCode(code)
}

func (s *fileSystem) Fsync(_ string, _ bool, fh uint64) int {
	//log.Warn().Uint64("Fd", fh).Msg("Fsync")
	code := s.session.CmdSync(uint32(fh))
	return errnoFromCode(code)
}

/*
func (s *fileSystem) Opendir(path string) (int, uint64) {
	const O_DIRECTORY = 0x10000
	fd, code := s.session.CmdOpen(path, uint32(os.O_RDONLY|O_DIRECTORY), 0)
	return errnoFromCode(code), uint64(fd)
}

func (s *fileSystem) Releasedir(_ string, fh uint64) int {
	code := s.session.CmdClose(uint32(fh))
	return errnoFromCode(code)
}
*/

func (s *fileSystem) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, _ int64, _ uint64) int {
	cacheKey := dirCacheKey(path)

	if path[len(path)-1] != '/' {
		path = path + "/"
	}

	var varfi wsfsprotocol.FileInfo
	var items []session.DirItem
	var code uint8

	cache, ok := s.cache.Get(cacheKey)
	if ok && cache.items != nil {
		varfi = cache.attr
		items = cache.items
	} else {
		if pendingCh, ok := s.loadPendingDir(cacheKey); ok {
			<-pendingCh
			cache, ok = s.cache.Get(cacheKey)
			if ok && cache.items != nil {
				varfi = cache.attr
				items = cache.items
			} else {
				s.pending.Delete(cacheKey)
			}
		}
	}

	if items == nil {
		//log.Warn().Str("Path", path).Msg("Readdir")
		varfi, code = s.session.CmdGetAttr(path)
		if code != wsfsprotocol.ErrorOK {
			return errnoFromCode(code)
		}

		items, code = s.session.CmdReadDirPlus(path)
		if code != wsfsprotocol.ErrorOK {
			items, code = s.session.CmdReadDir(path)
			if code != wsfsprotocol.ErrorOK {
				return errnoFromCode(code)
			}
		}

		for i := range items {
			item := &items[i]
			if pendingCh := item.PrefetchReady(); pendingCh != nil {
				childPath := path + item.Name + "/"
				s.storePendingDir(dirCacheKey(childPath), pendingCh)
			}
		}
		s.cachePrefetchedDirChildren(path, items)

		s.cache.Set(cacheKey, cachedData{attr: varfi, items: items})
	}

	stat := fuse.Stat_t{}
	statFromFileInfo(&stat, &varfi)
	fill(".", &stat, 0)
	fill("..", nil, 0)
	for _, item := range items {
		stat = fuse.Stat_t{}
		statFromDirItem(&stat, &item)
		fill(item.Name, &stat, 0)
	}

	return fuseOK
}

func (s *fileSystem) Destroy() {
	s.onDestroy()
}
