package unix

import (
	"syscall"
	"time"
	"wsfs-core/internal/client/session"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const zeroTimeDuration = time.Duration(0)

var _ = (fusefs.DirStream)((*dirListStream)(nil))

type dirListStream struct {
	items []session.DirItem
}

func (d *dirListStream) HasNext() bool {
	return len(d.items) > 0
}

func (d *dirListStream) Next() (e fuse.DirEntry, err syscall.Errno) {
	err = syscall.Errno(0)
	e.Name = d.items[0].Name
	e.Mode = d.items[0].Mode
	e.Ino = 0
	d.items = d.items[1:]
	return
}

func (d *dirListStream) Close() {
}
