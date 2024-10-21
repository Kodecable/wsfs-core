package unix

import (
	"sync/atomic"
	"time"
	"wsfs-core/internal/client/session"

	"github.com/hanwen/go-fuse/v2/fs"
)

var _ = (fs.DirStream)((*dirStream)(nil))

type dirStream struct {
	items []session.DirItem
}

type dirCache struct {
	items    []session.DirItem
	expireAt time.Time
}

func saveDirCache(pointer *atomic.Pointer[dirCache], items []session.DirItem, expire time.Duration) {
	pointer.Store(&dirCache{items: items, expireAt: time.Now().Add(expire)})
}

func lookupDirCache(pointer *atomic.Pointer[dirCache], name string) (session.DirItem, bool) {
	cache := pointer.Load()
	if cache == nil {
		return session.DirItem{}, false
	}
	if cache.expireAt.Before(time.Now()) {
		pointer.Store(nil)
		return session.DirItem{}, false
	}
	for _, item := range cache.items {
		if item.Name == name {
			return item, true
		}
	}
	return session.DirItem{}, false
}
