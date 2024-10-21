//go:build unix

package unix

import (
	"hash/maphash"
	"time"
	"wsfs-core/internal/client/session"

	"github.com/hanwen/go-fuse/v2/fs"
)

const fsBlockSize = 4096
const fsFileNameLen = 255

var inodeHashSeed = maphash.MakeSeed()

type fsRoot struct {
	session    *session.Session
	mountpoint string
	timeout    time.Duration
	suser      Suser_t
}

func NewRoot(sesseion *session.Session, suser Suser_t, timeout time.Duration) *fsRoot {
	return &fsRoot{
		session: sesseion,
		suser:   suser,
		timeout: timeout,
	}
}

func (r *fsRoot) NewNode() fs.InodeEmbedder {
	return &fsNode{RootData: r}
}
