//go:build unix

package unix

import (
	"context"
	"hash/maphash"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type fsNode struct {
	fs.Inode

	RootData *fsRoot
}

func (n *fsNode) path() string {
	return "/" + n.Path(n.Root())
}

var _ = (fs.NodeStatfser)((*fsNode)(nil))

func (n *fsNode) Statfs(_ context.Context, out *fuse.StatfsOut) syscall.Errno {
	fsi, code := n.RootData.session.CmdFsStat(n.path())
	if code != wsfsprotocol.ErrorOK {
		return errorCodeMap[code]
	}

	out.Frsize = fsBlockSize
	out.Bsize = fsBlockSize
	out.Blocks = fsi.Total / fsBlockSize
	out.Bfree = fsi.Free / fsBlockSize
	out.Bavail = fsi.Available / fsBlockSize
	out.NameLen = fsFileNameLen
	out.Files = 0
	out.Ffree = 0
	return fs.OK
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

func attrFromFi(path string, attr *fuse.Attr, fi *session.FileInfo, suser *Suser_t) {
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
		attr.Uid = suser.NobodyUid
		attr.Gid = suser.NobodyGid
	case wsfsprotocol.OWNER_NG:
		attr.Uid = suser.NobodyUid
		attr.Gid = suser.Gid
	case wsfsprotocol.OWNER_UN:
		attr.Uid = suser.Uid
		attr.Gid = suser.NobodyGid
	case wsfsprotocol.OWNER_UG:
		attr.Uid = suser.Uid
		attr.Gid = suser.Gid
	}
}

func idFromStat(attr *fuse.Attr) fs.StableAttr {
	return fs.StableAttr{
		Mode: attr.Mode,
		Ino:  attr.Ino,
		Gen:  1,
	}
}

var _ = (fs.NodeLookuper)((*fsNode)(nil))

func (n *fsNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	p := filepath.Join(n.path(), name)
	fi, code := n.RootData.session.CmdGetAttr(p)
	if code != wsfsprotocol.ErrorOK {
		return nil, errorCodeMap[code]
	}

	attrFromFi(p, &out.Attr, &fi, &n.RootData.suser)

	node := n.RootData.NewNode()
	ch := n.NewInode(ctx, node, idFromStat(&out.Attr))
	return ch, fs.OK
}

var _ = (fs.NodeMkdirer)((*fsNode)(nil))

func (n *fsNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	code := n.RootData.session.CmdMkdir(filepath.Join(n.path(), name), mode)
	if code != wsfsprotocol.ErrorOK {
		return nil, errorCodeMap[code]
	}

	return n.Lookup(ctx, name, out) // TODO: handle lookup error
}

var _ = (fs.NodeRmdirer)((*fsNode)(nil))

func (n *fsNode) Rmdir(_ context.Context, name string) syscall.Errno {
	code := n.RootData.session.CmdRmDir(filepath.Join(n.path(), name))
	return errorCodeMap[code]
}

var _ = (fs.NodeUnlinker)((*fsNode)(nil))

func (n *fsNode) Unlink(_ context.Context, name string) syscall.Errno {
	code := n.RootData.session.CmdRemove(filepath.Join(n.path(), name))
	return errorCodeMap[code]
}

var _ = (fs.NodeRenamer)((*fsNode)(nil))

func (n *fsNode) Rename(_ context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	p1 := filepath.Join(n.path(), name)
	p2 := filepath.Join("/"+newParent.EmbeddedInode().Path(nil), newName)
	code := n.RootData.session.CmdRename(p1, p2, flags)
	return errorCodeMap[code]
}

var _ = (fs.NodeCreater)((*fsNode)(nil))

func (n *fsNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	p := filepath.Join(n.path(), name)
	fd, code := n.RootData.session.CmdOpen(p, flags|uint32(os.O_CREATE), mode)
	if code != wsfsprotocol.ErrorOK {
		return nil, nil, 0, errorCodeMap[code]
	}

	in, err := n.Lookup(ctx, name, out)
	if err != syscall.Errno(0) {
		n.RootData.session.CmdClose(fd)
		return nil, nil, 0, err
	}

	return in, fd, 0, 0
}

var _ = (fs.NodeSymlinker)((*fsNode)(nil))

func (n *fsNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if !strings.HasPrefix(target, n.RootData.mountpoint) {
		return nil, syscall.EACCES
	}
	target = strings.TrimPrefix(target, n.RootData.mountpoint)

	p := filepath.Join(n.path(), name)
	code := n.RootData.session.CmdSymLink(target, p)
	if code != wsfsprotocol.ErrorOK {
		return nil, errorCodeMap[code]
	}

	return n.Lookup(ctx, name, out)
}

var _ = (fs.NodeLinker)((*fsNode)(nil))

func (n *fsNode) Link(_ context.Context, _ fs.InodeEmbedder, _ string, _ *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	return nil, syscall.ENOTSUP
}

var _ = (fs.NodeReadlinker)((*fsNode)(nil))

func (n *fsNode) Readlink(_ context.Context) ([]byte, syscall.Errno) {
	p := n.path()

	path, code := n.RootData.session.CmdReadLink(p)
	return []byte(path), errorCodeMap[code]
}

var _ = (fs.NodeOpener)((*fsNode)(nil))

func (n *fsNode) Open(_ context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	fd, code := n.RootData.session.CmdOpen(n.path(), flags, 0o644)
	return fd, 0, errorCodeMap[code]
}

var _ = (fs.NodeOpendirer)((*fsNode)(nil))

func (n *fsNode) Opendir(_ context.Context) syscall.Errno {
	return fs.OK
}

var _ = (fs.DirStream)((*dirStream)(nil))

type dirStream struct {
	items []session.DirItem
}

func (d *dirStream) HasNext() bool {
	return len(d.items) > 0
}

func (d *dirStream) Next() (e fuse.DirEntry, err syscall.Errno) {
	err = syscall.Errno(0)
	e.Name = d.items[0].Name
	e.Mode = d.items[0].Mode
	e.Ino = 0
	d.items = d.items[1:]
	return
}

func (d *dirStream) Close() {
}

var _ = (fs.NodeReaddirer)((*fsNode)(nil))

func (n *fsNode) Readdir(_ context.Context) (fs.DirStream, syscall.Errno) {
	items, code := n.RootData.session.CmdReadDir(n.path())
	if code != wsfsprotocol.ErrorOK {
		return nil, errorCodeMap[code]
	}
	return &dirStream{items: items}, fs.OK
}

var _ = (fs.NodeGetattrer)((*fsNode)(nil))

func (n *fsNode) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	p := n.path()

	fi, code := n.RootData.session.CmdGetAttr(p)
	if code != wsfsprotocol.ErrorOK {
		return errorCodeMap[code]
	}
	attrFromFi(p, &out.Attr, &fi, &n.RootData.suser)

	return fs.OK
}

var _ = (fs.NodeReader)((*fsNode)(nil))

func (n *fsNode) Read(_ context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	readed, code := n.RootData.session.CmdReadAt(f.(uint32), uint64(off), dest)
	if code != wsfsprotocol.ErrorOK {
		return nil, errorCodeMap[code]
	}

	return fuse.ReadResultData(dest[:readed]), fs.OK
}

var _ = (fs.NodeWriter)((*fsNode)(nil))

func (n *fsNode) Write(_ context.Context, f fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	count, code := n.RootData.session.CmdWriteAt(f.(uint32), uint64(off), data)
	if code != wsfsprotocol.ErrorOK {
		return 0, errorCodeMap[code]
	}

	return uint32(count), fs.OK
}

var _ = (fs.NodeFlusher)((*fsNode)(nil))

func (n *fsNode) Flush(_ context.Context, _ fs.FileHandle) syscall.Errno {
	return fs.OK
}

var _ = (fs.NodeReleaser)((*fsNode)(nil))

func (n *fsNode) Release(_ context.Context, f fs.FileHandle) syscall.Errno {
	_ = n.RootData.session.CmdClose(f.(uint32)) // ignore error
	return fs.OK
}

var _ = (fs.NodeFsyncer)((*fsNode)(nil))

func (n *fsNode) Fsync(_ context.Context, f fs.FileHandle, flags uint32) syscall.Errno {
	code := n.RootData.session.CmdSync(f.(uint32))
	return errorCodeMap[code]
}

var _ = (fs.NodeLseeker)((*fsNode)(nil))

func (n *fsNode) Lseek(_ context.Context, f fs.FileHandle, Off uint64, whence uint32) (uint64, syscall.Errno) {
	off, code := n.RootData.session.CmdSeek(f.(uint32), whence, int64(Off))
	return off, errorCodeMap[code]
}

var _ = (fs.NodeCopyFileRanger)((*fsNode)(nil))

func (n *fsNode) CopyFileRange(_ context.Context, fhIn fs.FileHandle, offIn uint64, out *fs.Inode, fhOut fs.FileHandle, offOut uint64, len uint64, flags uint64) (uint32, syscall.Errno) {
	if flags != 0 {
		return 0, syscall.ENOTSUP
	}
	copyed, code := n.RootData.session.CmdCopyFileRange(fhIn.(uint32), fhOut.(uint32), offIn, offOut, len)
	return uint32(copyed), errorCodeMap[code]
}

var _ = (fs.NodeSetattrer)((*fsNode)(nil))

func (n *fsNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	var flag uint8
	var fi session.FileInfo
	var orig fuse.AttrOut

	err := n.Getattr(ctx, f, &orig)
	if err != fs.OK {
		return err
	}

	if m, ok := in.GetMode(); ok {
		flag |= wsfsprotocol.SETATTR_MODE
		fi.Mode = m
	}

	flag |= wsfsprotocol.SETATTR_OWNER
	if uid, ok := in.GetUID(); ok {
		if uid == n.RootData.suser.Uid {
			fi.Owner += 1
		}
	} else if orig.Uid == n.RootData.suser.Uid {
		fi.Owner += 1
	}
	if gid, ok := in.GetGID(); ok {
		if gid == n.RootData.suser.Gid {
			fi.Owner += 2
		}
	} else if orig.Gid == n.RootData.suser.Gid {
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
		code = n.RootData.session.CmdSetAttrByFD(fd, flag, fi)
	} else {
		code = n.RootData.session.CmdSetAttr(n.path(), flag, fi)
	}
	return errorCodeMap[code]
}

var _ = (fs.NodeGetlker)((*fsNode)(nil))

func (n *fsNode) Getlk(_ context.Context, _ fs.FileHandle, _ uint64, _ *fuse.FileLock, _ uint32, _ *fuse.FileLock) syscall.Errno {
	return syscall.ENOTSUP
}

var _ = (fs.NodeSetlker)((*fsNode)(nil))

func (n *fsNode) Setlk(_ context.Context, _ fs.FileHandle, _ uint64, _ *fuse.FileLock, _ uint32) syscall.Errno {
	return syscall.ENOTSUP
}

var _ = (fs.NodeSetlkwer)((*fsNode)(nil))

func (n *fsNode) Setlkw(_ context.Context, _ fs.FileHandle, _ uint64, _ *fuse.FileLock, _ uint32) syscall.Errno {
	return syscall.ENOTSUP
}

var _ = (fs.NodeGetxattrer)((*fsNode)(nil))

func (n *fsNode) Getxattr(_ context.Context, _ string, _ []byte) (uint32, syscall.Errno) {
	return 0, syscall.ENOTSUP
}

var _ = (fs.NodeSetxattrer)((*fsNode)(nil))

func (n *fsNode) Setxattr(_ context.Context, _ string, _ []byte, _ uint32) syscall.Errno {
	return syscall.ENOTSUP
}

var _ = (fs.NodeRemovexattrer)((*fsNode)(nil))

func (n *fsNode) Removexattr(_ context.Context, _ string) syscall.Errno {
	return syscall.ENOTSUP
}

var _ = (fs.NodeListxattrer)((*fsNode)(nil))

func (n *fsNode) Listxattr(_ context.Context, _ []byte) (uint32, syscall.Errno) {
	return 0, syscall.ENOTSUP
}

var _ = (fs.NodeMknoder)((*fsNode)(nil))

func (n *fsNode) Mknod(_ context.Context, _ string, _ uint32, _ uint32, _ *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	return nil, syscall.ENOTSUP
}
