package unix

import (
	"sync/atomic"
	"time"
	"wsfs-core/internal/client/session"
)

type dirCache struct {
	items    []session.DirItem
	expireAt time.Time
}

type dataCache struct {
	data     []byte
	expireAt time.Time
}

type attrCache struct {
	attr     session.FileInfo
	expireAt time.Time
}

func subDirCache(pointer *atomic.Pointer[dirCache], items []session.DirItem, father *atomic.Pointer[dirCache]) {
	pointer.Store(&dirCache{items: items, expireAt: father.Load().expireAt})
}

func subDataCache(pointer *atomic.Pointer[dataCache], data []byte, father *atomic.Pointer[dirCache]) {
	pointer.Store(&dataCache{data: data, expireAt: father.Load().expireAt})
}

func subAttrCache(pointer *atomic.Pointer[attrCache], attr session.FileInfo, father *atomic.Pointer[dirCache]) {
	pointer.Store(&attrCache{attr: attr, expireAt: father.Load().expireAt})
}

func saveDirCache(pointer *atomic.Pointer[dirCache], items []session.DirItem, expire time.Duration) {
	pointer.Store(&dirCache{items: items, expireAt: time.Now().Add(expire)})
}

func saveAttrCache(pointer *atomic.Pointer[attrCache], attr session.FileInfo, expire time.Duration) {
	pointer.Store(&attrCache{attr: attr, expireAt: time.Now().Add(expire)})
}

// if ok is false, cache is expired
// if not found, return empty(invaild) DirItem which have a empty name
func lookupDirCache(pointer *atomic.Pointer[dirCache], name string) (fi session.DirItem, delta time.Duration, ok bool) {
	cache := pointer.Load()
	if cache == nil || cache.items == nil {
		return session.DirItem{}, zeroTimeDuration, false
	}

	delta = time.Until(cache.expireAt)
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
	if cache == nil || cache.items == nil {
		return nil, false
	}
	if cache.expireAt.Before(time.Now()) {
		pointer.Store(nil)
		return nil, false
	}
	return cache.items, true
}

// return data even if expire
func getDataCache(pointer *atomic.Pointer[dataCache]) ([]byte, bool) {
	cache := pointer.Load()
	if cache == nil || cache.data == nil {
		return nil, false
	}
	if cache.expireAt.Before(time.Now()) {
		return cache.data, false
	}
	return cache.data, true
}

func getAttrCache(pointer *atomic.Pointer[attrCache]) (session.FileInfo, bool) {
	cache := pointer.Load()
	if cache == nil {
		return session.FileInfo{}, false
	}
	if cache.expireAt.Before(time.Now()) {
		pointer.Store(nil)
		return session.FileInfo{}, false
	}
	return cache.attr, true
}

func wipeDirCache(pointer *atomic.Pointer[dirCache]) {
	pointer.Store(nil)
}

// not remove data, someone may still opend it.
func wipeDataCache(pointer *atomic.Pointer[dataCache]) {
	p := pointer.Load()
	if p == nil {
		return
	}
	pointer.Store(&dataCache{data: p.data, expireAt: time.Now()})
}

func wipeAttrCache(pointer *atomic.Pointer[attrCache]) {
	pointer.Store(nil)
}
