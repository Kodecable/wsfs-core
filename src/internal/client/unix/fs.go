//go:build unix

package unix

import (
	"hash/maphash"
	"time"
	"wsfs-core/internal/client/session"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
)

const fsBlockSize = 4096
const fsFileNameLen = 255

var inodeHashSeed = maphash.MakeSeed()

type fileSystem struct {
	session *session.Session

	mountpoint string
	suser      Suser_t

	structTimeout   time.Duration
	negativeTimeout time.Duration
}

func NewFS(sesseion *session.Session, suser Suser_t,
	structTimeout, negativeTimeout time.Duration) *fileSystem {
	return &fileSystem{
		session:         sesseion,
		suser:           suser,
		structTimeout:   structTimeout,
		negativeTimeout: negativeTimeout,
	}
}

func (r *fileSystem) NewNode() fusefs.InodeEmbedder {
	return &fsNode{fsdata: r}
}
