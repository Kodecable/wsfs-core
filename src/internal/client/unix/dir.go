package unix

import (
	"sync/atomic"
	"syscall"
	"time"
	"wsfs-core/internal/client/session"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const zeroTimeDuration = time.Duration(0)

var _ = (fusefs.DirStream)((*dirStream)(nil))

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

type dirCache struct {
	items    []session.DirItem
	expireAt time.Time
}

func saveDirCache(pointer *atomic.Pointer[dirCache], items []session.DirItem, expire time.Duration) {
	pointer.Store(&dirCache{items: items, expireAt: time.Now().Add(expire)})
}

func lookupDirCache(pointer *atomic.Pointer[dirCache], name string) (session.DirItem, time.Duration, bool) {
	cache := pointer.Load()
	if cache == nil {
		return session.DirItem{}, zeroTimeDuration, false
	}

	delta := time.Until(cache.expireAt)
	if delta < 0 {
		pointer.Store(nil)
		return session.DirItem{}, zeroTimeDuration, false
	}

	for _, item := range cache.items {
		if item.Name == name {
			return item, delta, true
		}
	}
	return session.DirItem{}, delta, true
}

func getDirCache(pointer *atomic.Pointer[dirCache]) ([]session.DirItem, bool) {
	cache := pointer.Load()
	if cache == nil {
		return []session.DirItem{}, false
	}
	if cache.expireAt.Before(time.Now()) {
		pointer.Store(nil)
		return []session.DirItem{}, false
	}
	return cache.items, true
}

func wipeDirCache(pointer *atomic.Pointer[dirCache]) {
	pointer.Store(nil)
}
