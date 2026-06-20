//go:build unix

package unix

import (
	"context"
	"hash/maphash"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/share/wsfsunixconv"
	"wsfs-core/internal/util"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

type fsNode struct {
	fusefs.Inode

	fsdata    *fileSystem
	dirCache  atomic.Pointer[dirCache]
	dataCache atomic.Pointer[dataCache]
	attrCache atomic.Pointer[attrCache]
}

func (n *fsNode) path() string {
	return "/" + n.Path(n.Root())
}

var _ = (fusefs.NodeStatfser)((*fsNode)(nil))

func (n *fsNode) Statfs(_ context.Context, out *fuse.StatfsOut) syscall.Errno {
	fsi, code := n.fsdata.session.CmdFsStat(n.path())
	if code != wsfsprotocol.ErrorOK {
		return errnoFromCode(code)
	}

	out.Frsize = fsBlockSize
	out.Bsize = fsBlockSize
	out.Blocks = fsi.Total / fsBlockSize
	out.Bfree = fsi.Free / fsBlockSize
	out.Bavail = fsi.Available / fsBlockSize
	out.NameLen = fsFileNameLen
	out.Files = 0
	out.Ffree = 0
	return fusefs.OK
}

func fileMode(mode uint32) (r uint32) {
	r = mode & 0o777

	if mode&uint32(os.ModeSetuid) != 0 {
		r |= syscall.S_ISUID
	}
	if mode&uint32(os.ModeSetgid) != 0 {
		r |= syscall.S_ISGID
	}
	if mode&uint32(os.ModeSticky) != 0 {
		r |= syscall.S_ISVTX
	}

	if mode&uint32(os.ModeDir) != 0 {
		r |= syscall.S_IFDIR
	} else if mode&uint32(os.ModeSymlink) != 0 {
		r |= syscall.S_IFLNK
	} else if mode&(uint32(os.ModeDevice)|
		uint32(os.ModeNamedPipe)|
		uint32(os.ModeSocket)) != 0 {
		return r & ^uint32(0o777)
	} else {
		r |= syscall.S_IFREG
	}

	return
}

func attrFromFileInfo(path string, attr *fuse.Attr, fi *wsfsprotocol.FileInfo, fsIds *util.FsIds) {
	attr.Ino = maphash.String(inodeHashSeed, path)
	attr.Size = fi.Size
	attr.Atime = uint64(fi.MTime)
	attr.Ctime = uint64(fi.MTime)
	attr.Mtime = uint64(fi.MTime)
	attr.Atimensec = 0
	attr.Ctimensec = 0
	attr.Mtimensec = 0
	attr.Mode = fileMode(fi.Mode)
	attr.Nlink = 1
	attr.Blksize = fsBlockSize

	attr.Blocks = uint64(fi.Size / 512)
	if fi.Size%512 != 0 {
		attr.Blocks += 1
	}

	switch fi.Owner {
	case wsfsprotocol.OWNER_NN:
		attr.Uid = fsIds.OtherUid
		attr.Gid = fsIds.OtherGid
	case wsfsprotocol.OWNER_NG:
		attr.Uid = fsIds.OtherUid
		attr.Gid = fsIds.Gid
	case wsfsprotocol.OWNER_UN:
		attr.Uid = fsIds.Uid
		attr.Gid = fsIds.OtherGid
	case wsfsprotocol.OWNER_UG:
		attr.Uid = fsIds.Uid
		attr.Gid = fsIds.Gid
	}
}

func attrFromDirItem(path string, attr *fuse.Attr, di *session.DirItem, fsIds *util.FsIds) {
	attrFromFileInfo(path, attr, &wsfsprotocol.FileInfo{
		Size:  di.Size,
		MTime: di.MTime,
		Mode:  di.Mode,
		Owner: di.Owner,
	}, fsIds)
}

func idFromStat(attr *fuse.Attr) fusefs.StableAttr {
	return fusefs.StableAttr{
		Mode: attr.Mode,
		Ino:  attr.Ino,
		Gen:  1,
	}
}

var _ = (fusefs.NodeLookuper)((*fsNode)(nil))

func (n *fsNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fusefs.Inode, syscall.Errno) {
	p := filepath.Join(n.path(), name)

	item, timeoutDelta, ok := lookupDirCache(&n.dirCache, name)
	if ok {
		//log.Debug().Str("Path", p).Msg("Lookup cached entry")
		out.SetAttrTimeout(timeoutDelta)
		out.SetEntryTimeout(timeoutDelta)
		if item != nil {
			attrFromDirItem(p, &out.Attr, item, &n.fsdata.fsIds)
		} else {
			return nil, syscall.ENOENT
		}
	} else {
		//log.Debug().Str("Path", p).Msg("Lookup entry")
		fi, code := n.fsdata.session.CmdGetAttr(p)
		if code != wsfsprotocol.ErrorOK {
			return nil, errnoFromCode(code)
		}
		attrFromFileInfo(p, &out.Attr, &fi, &n.fsdata.fsIds)
	}

	node := n.fsdata.NewNode()
	childNode := node.(*fsNode)
	if ok && item != nil {
		// Wait for prefetch data from CmdReadDirPlus background goroutine
		item.WaitPrefetch()
		if item.Child != nil {
			subDirCache(&childNode.dirCache, item.Child, &n.dirCache)
		} else if item.Data != nil {
			subDataCache(&childNode.dataCache, item.Data, &n.dirCache)
			subAttrCache(&childNode.attrCache, wsfsprotocol.FileInfo{
				Size:  item.Size,
				MTime: item.MTime,
				Mode:  item.Mode,
				Owner: item.Owner,
			}, &n.dirCache)
		}
	}

	ch := n.NewInode(ctx, node, idFromStat(&out.Attr))
	if ch == nil {
		log.Error().Str("Path", p).Msg("NewInode returned nil in Lookup")
		return nil, syscall.EIO
	}
	return ch, fusefs.OK
}

var _ = (fusefs.NodeMkdirer)((*fsNode)(nil))

func (n *fsNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fusefs.Inode, syscall.Errno) {
	wipeDirCache(&n.dirCache)
	code := n.fsdata.session.CmdMkdir(filepath.Join(n.path(), name), mode)
	if code != wsfsprotocol.ErrorOK {
		return nil, errnoFromCode(code)
	}

	return n.Lookup(ctx, name, out) // TODO: handle lookup error
}

var _ = (fusefs.NodeRmdirer)((*fsNode)(nil))

func (n *fsNode) Rmdir(_ context.Context, name string) syscall.Errno {
	wipeDirCache(&n.dirCache)
	code := n.fsdata.session.CmdRmDir(filepath.Join(n.path(), name))
	return errnoFromCode(code)
}

var _ = (fusefs.NodeUnlinker)((*fsNode)(nil))

func (n *fsNode) Unlink(_ context.Context, name string) syscall.Errno {
	wipeDirCache(&n.dirCache)
	code := n.fsdata.session.CmdRemove(filepath.Join(n.path(), name))
	return errnoFromCode(code)
}

var _ = (fusefs.NodeRenamer)((*fsNode)(nil))

func (n *fsNode) Rename(_ context.Context, name string, newParent fusefs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	wipeDirCache(&n.dirCache)
	p1 := filepath.Join(n.path(), name)
	p2 := filepath.Join("/"+newParent.EmbeddedInode().Path(nil), newName)

	if flags & ^wsfsunixconv.AcceptedUnixRenameFlags != 0 {
		return syscall.ENOTSUP
	}

	var wsfsFlags uint32 = 0
	for unixFlag, wsfsFlag := range wsfsunixconv.RenameFlagFromUnix {
		if flags&unixFlag != 0 {
			wsfsFlags |= wsfsFlag
		}
	}

	code := n.fsdata.session.CmdRename(p1, p2, wsfsFlags)
	return errnoFromCode(code)
}

func OpenFlagFromSys(in int) (out uint32, ok bool) {
	in &= ^wsfsunixconv.IgnoredUnixOpenFlagBits
	if in & ^wsfsunixconv.AcceptdUnixOpenFlagBits != 0 {
		return 0, false
	}

	switch in & unix.O_ACCMODE {
	case unix.O_RDONLY:
		out |= wsfsprotocol.O_RDONLY
	case unix.O_WRONLY:
		out |= wsfsprotocol.O_WRONLY
	case unix.O_RDWR:
		out |= wsfsprotocol.O_RDWR
	default:
		return 0, false
	}

	for unixFlag, wsfsFlag := range wsfsunixconv.OpenFlagFromUnix {
		if unixFlag&unix.O_ACCMODE != 0 {
			continue
		}
		if in&unixFlag != 0 {
			out |= wsfsFlag
		}
	}
	return out, true
}

var _ = (fusefs.NodeCreater)((*fsNode)(nil))

func (n *fsNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (inode *fusefs.Inode, fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	wipeDirCache(&n.dirCache)
	p := filepath.Join(n.path(), name)

	wsfsFlag, ok := OpenFlagFromSys(int(flags))
	if !ok {
		return nil, nil, 0, syscall.ENOTSUP
	}

	fd, code := n.fsdata.session.CmdOpen(p, wsfsFlag|wsfsprotocol.O_CREAT, mode)
	if code != wsfsprotocol.ErrorOK {
		return nil, nil, 0, errnoFromCode(code)
	}

	in, err := n.Lookup(ctx, name, out)
	if err != syscall.Errno(0) {
		n.fsdata.session.CmdClose(fd)
		return nil, nil, 0, err
	}

	return in, fd, 0, 0
}

var _ = (fusefs.NodeSymlinker)((*fsNode)(nil))

func (n *fsNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fusefs.Inode, syscall.Errno) {
	wipeDirCache(&n.dirCache)
	if !strings.HasPrefix(target, n.fsdata.mountpoint) {
		return nil, syscall.EACCES
	}
	target = strings.TrimPrefix(target, n.fsdata.mountpoint)

	p := filepath.Join(n.path(), name)
	code := n.fsdata.session.CmdSymLink(target, p)
	if code != wsfsprotocol.ErrorOK {
		return nil, errnoFromCode(code)
	}

	return n.Lookup(ctx, name, out)
}

var _ = (fusefs.NodeLinker)((*fsNode)(nil))

func (n *fsNode) Link(_ context.Context, _ fusefs.InodeEmbedder, _ string, _ *fuse.EntryOut) (*fusefs.Inode, syscall.Errno) {
	return nil, syscall.ENOTSUP
}

var _ = (fusefs.NodeReadlinker)((*fsNode)(nil))

func (n *fsNode) Readlink(_ context.Context) ([]byte, syscall.Errno) {
	p := n.path()

	path, code := n.fsdata.session.CmdReadLink(p)
	if code != wsfsprotocol.ErrorOK {
		return nil, errnoFromCode(code)
	}
	return []byte(filepath.Join(n.fsdata.mountpoint, strings.TrimPrefix(path, "/"))), fusefs.OK
}

var _ = (fusefs.NodeOpener)((*fsNode)(nil))

func (n *fsNode) Open(_ context.Context, flags uint32) (fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if flags == uint32(os.O_RDONLY) {
		if _, ok := getDataCache(&n.dataCache); ok {
			//log.Debug().Str("Path", n.path()).Msg("Open cached")
			return nil, 0, fusefs.OK
		}
	}
	//log.Debug().Str("Path", n.path()).Msg("Open")

	wsfsFlag, ok := OpenFlagFromSys(int(flags))
	if !ok {
		return nil, 0, syscall.ENOTSUP
	}

	fd, code := n.fsdata.session.CmdOpen(n.path(), wsfsFlag, 0o644)
	return fd, 0, errnoFromCode(code)
}

var _ = (fusefs.NodeOpendirer)((*fsNode)(nil))

func (n *fsNode) Opendir(_ context.Context) syscall.Errno {
	return fusefs.OK
}

var _ = (fusefs.NodeReaddirer)((*fsNode)(nil))

func (n *fsNode) Readdir(_ context.Context) (fusefs.DirStream, syscall.Errno) {
	items, ok := getDirCache(&n.dirCache)
	if ok {
		// TODO: pass timeout delta to fuse
		//log.Debug().Str("Path", n.path()).Msg("Read cached dir")
		return &dirListStream{items: items}, fusefs.OK
	}

	path := n.path()
	if path[len(path)-1] != '/' {
		path = path + "/"
	}

	//log.Debug().Str("Path", n.path()).Msg("Read dir")

	items, code := n.fsdata.session.CmdReadDirPlus(path)
	if code != wsfsprotocol.ErrorOK {
		items, code = n.fsdata.session.CmdReadDir(path)
		if code != wsfsprotocol.ErrorOK {
			return nil, errnoFromCode(code)
		}
	}
	saveDirCache(&n.dirCache, items, n.fsdata.structTimeout)
	return &dirListStream{items: items}, fusefs.OK
}

var _ = (fusefs.NodeGetattrer)((*fsNode)(nil))

func (n *fsNode) Getattr(_ context.Context, _ fusefs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	p := n.path()

	fi, ok := getAttrCache(&n.attrCache)
	if ok {
		attrFromFileInfo(p, &out.Attr, &fi, &n.fsdata.fsIds)
		return fusefs.OK
	}

	fi, code := n.fsdata.session.CmdGetAttr(p)
	if code != wsfsprotocol.ErrorOK {
		return errnoFromCode(code)
	}
	attrFromFileInfo(p, &out.Attr, &fi, &n.fsdata.fsIds)
	saveAttrCache(&n.attrCache, fi, n.fsdata.structTimeout)

	return fusefs.OK
}

var _ = (fusefs.NodeReader)((*fsNode)(nil))

func (n *fsNode) Read(_ context.Context, f fusefs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if f == nil {
		data, _ := getDataCache(&n.dataCache)
		return fuse.ReadResultData(data[off:]), fusefs.OK
	}

	readed, code := n.fsdata.session.CmdReadAt(f.(uint32), uint64(off), dest)
	if code != wsfsprotocol.ErrorOK {
		return nil, errnoFromCode(code)
	}

	return fuse.ReadResultData(dest[:readed]), fusefs.OK
}

var _ = (fusefs.NodeWriter)((*fsNode)(nil))

func (n *fsNode) Write(_ context.Context, f fusefs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	if f == nil {
		return 0, syscall.EINVAL
	}
	wipeDataCache(&n.dataCache)
	wipeAttrCache(&n.attrCache)

	count, code := n.fsdata.session.CmdWriteAt(f.(uint32), uint64(off), data)
	if code != wsfsprotocol.ErrorOK {
		return 0, errnoFromCode(code)
	}

	return uint32(count), fusefs.OK
}

var _ = (fusefs.NodeFlusher)((*fsNode)(nil))

func (n *fsNode) Flush(_ context.Context, _ fusefs.FileHandle) syscall.Errno {
	wipeDataCache(&n.dataCache)
	wipeAttrCache(&n.attrCache)
	return fusefs.OK
}

var _ = (fusefs.NodeReleaser)((*fsNode)(nil))

func (n *fsNode) Release(_ context.Context, f fusefs.FileHandle) syscall.Errno {
	if f == nil {
		return fusefs.OK
	}

	_ = n.fsdata.session.CmdClose(f.(uint32)) // ignore error
	return fusefs.OK
}

var _ = (fusefs.NodeFsyncer)((*fsNode)(nil))

func (n *fsNode) Fsync(_ context.Context, f fusefs.FileHandle, flags uint32) syscall.Errno {
	if f == nil {
		return fusefs.OK
	}
	wipeDataCache(&n.dataCache)
	wipeAttrCache(&n.attrCache)

	code := n.fsdata.session.CmdSync(f.(uint32))
	return errnoFromCode(code)
}

var _ = (fusefs.NodeLseeker)((*fsNode)(nil))

func (n *fsNode) Lseek(_ context.Context, f fusefs.FileHandle, Off uint64, whence uint32) (uint64, syscall.Errno) {
	if f == nil {
		return 0, fusefs.OK
	}

	wsfsWhence, ok := wsfsunixconv.WhenceFromUnix[int(whence)]
	if !ok {
		return 0, syscall.ENOTSUP
	}

	off, code := n.fsdata.session.CmdSeek(f.(uint32), wsfsWhence, int64(Off))
	return off, errnoFromCode(code)
}

var _ = (fusefs.NodeCopyFileRanger)((*fsNode)(nil))

func (n *fsNode) CopyFileRange(_ context.Context, fhIn fusefs.FileHandle, offIn uint64, out *fusefs.Inode, fhOut fusefs.FileHandle, offOut uint64, len uint64, flags uint64) (uint32, syscall.Errno) {
	if flags != 0 || fhIn == nil || fhOut == nil {
		return 0, syscall.ENOTSUP
	}
	wipeDataCache(&n.dataCache)
	wipeAttrCache(&n.attrCache)
	copyed, code := n.fsdata.session.CmdCopyFileRange(fhIn.(uint32), fhOut.(uint32), offIn, offOut, len)
	return uint32(copyed), errnoFromCode(code)
}

var _ = (fusefs.NodeSetattrer)((*fsNode)(nil))

func (n *fsNode) Setattr(ctx context.Context, f fusefs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	var flag uint8
	var fi wsfsprotocol.FileInfo
	var orig fuse.AttrOut

	err := n.Getattr(ctx, f, &orig)
	if err != fusefs.OK {
		return err
	}

	if m, ok := in.GetMode(); ok {
		flag |= wsfsprotocol.SETATTR_MODE
		fi.Mode = m
	}

	flag |= wsfsprotocol.SETATTR_OWNER
	if uid, ok := in.GetUID(); ok {
		if uid == n.fsdata.fsIds.Uid {
			fi.Owner += 1
		}
	} else if orig.Uid == n.fsdata.fsIds.Uid {
		fi.Owner += 1
	}
	if gid, ok := in.GetGID(); ok {
		if gid == n.fsdata.fsIds.Gid {
			fi.Owner += 2
		}
	} else if orig.Gid == n.fsdata.fsIds.Gid {
		fi.Owner += 2
	}

	if mtime, ok := in.GetMTime(); ok {
		flag |= wsfsprotocol.SETATTR_MTIME
		fi.MTime = mtime.Unix()
	}

	if size, ok := in.GetSize(); ok {
		flag |= wsfsprotocol.SETATTR_SIZE
		fi.Size = size
	}

	var code uint8
	if fd, ok := f.(uint32); ok {
		code = n.fsdata.session.CmdSetAttrByFD(fd, flag, fi)
	} else {
		code = n.fsdata.session.CmdSetAttr(n.path(), flag, fi)
	}
	if code == wsfsprotocol.ErrorOK {
		wipeDataCache(&n.dataCache)
		wipeAttrCache(&n.attrCache)
	}
	return errnoFromCode(code)
}

var _ = (fusefs.NodeGetxattrer)((*fsNode)(nil))

func (n *fsNode) Getxattr(_ context.Context, _ string, _ []byte) (uint32, syscall.Errno) {
	return 0, syscall.ENOTSUP
}

var _ = (fusefs.NodeSetxattrer)((*fsNode)(nil))

func (n *fsNode) Setxattr(_ context.Context, _ string, _ []byte, _ uint32) syscall.Errno {
	return syscall.ENOTSUP
}

var _ = (fusefs.NodeRemovexattrer)((*fsNode)(nil))

func (n *fsNode) Removexattr(_ context.Context, _ string) syscall.Errno {
	return syscall.ENOTSUP
}

var _ = (fusefs.NodeListxattrer)((*fsNode)(nil))

func (n *fsNode) Listxattr(_ context.Context, _ []byte) (uint32, syscall.Errno) {
	return 0, syscall.ENOTSUP
}

var _ = (fusefs.NodeMknoder)((*fsNode)(nil))

func (n *fsNode) Mknod(_ context.Context, _ string, _ uint32, _ uint32, _ *fuse.EntryOut) (*fusefs.Inode, syscall.Errno) {
	return nil, syscall.ENOTSUP
}
