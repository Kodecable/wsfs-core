package windows

import (
	"sync"
	"time"
	"wsfs-core/internal/client/session"
)

type cachedData struct {
	items []session.DirItem
	attr  session.FileInfo
	data  []byte
}

type cacheRecord struct {
	expireAt time.Time
	data     cachedData
}

type fsCache struct {
	cache   sync.Map
	timeout time.Duration
}

func (f *fsCache) Set(key string, val cachedData) {
	f.cache.Store(key, cacheRecord{expireAt: time.Now().Add(f.timeout), data: val})
}

func (f *fsCache) Del(key string) {
	f.cache.Delete(key)
}

func (f *fsCache) Get(key string) (val cachedData, ok bool) {
	recordV, ok := f.cache.Load(key)
	if !ok {
		return cachedData{}, false
	}
	record := recordV.(cacheRecord)

	delta := time.Until(record.expireAt)
	if delta < 0 {
		f.cache.Delete(key)
		return cachedData{}, false
	}

	return record.data, true
}

func (f *fsCache) Run() {
	for {
		time.Sleep(f.timeout)
		f.cache.Range(func(key, value any) bool {
			record := value.(cacheRecord)
			delta := time.Until(record.expireAt)
			if delta < 0 {
				f.cache.Delete(key)
			}
			return true
		})
	}
}
