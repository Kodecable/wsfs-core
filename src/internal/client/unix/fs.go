//go:build unix

package unix

import (
	"hash/maphash"
	"time"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/util"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
)

const fsBlockSize = 4096
const fsFileNameLen = 255

var inodeHashSeed = maphash.MakeSeed()

type fileSystem struct {
	session *session.Session

	mountpoint string
	fsIds      util.FsIds

	structTimeout   time.Duration
	negativeTimeout time.Duration
}

func NewFS(sesseion *session.Session, fsIds util.FsIds,
	mountpoint string, structTimeout, negativeTimeout time.Duration) *fileSystem {
	return &fileSystem{
		session:         sesseion,
		mountpoint:      mountpoint,
		fsIds:           fsIds,
		structTimeout:   structTimeout,
		negativeTimeout: negativeTimeout,
	}
}

func (r *fileSystem) NewNode() fusefs.InodeEmbedder {
	return &fsNode{fsdata: r}
}
