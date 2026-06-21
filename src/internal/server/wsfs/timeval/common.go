package timeval

import (
	"io/fs"
	"os"
	"time"

	"wsfs-core/internal/share/wsfsprotocol"
)

func FromTime(t time.Time) wsfsprotocol.Timespec {
	return wsfsprotocol.Timespec{
		Seconds:     t.Unix(),
		Nanoseconds: int64(t.Nanosecond()),
	}
}

func FromFileInfo(fi fs.FileInfo) wsfsprotocol.Timespec {
	return FromTime(fi.ModTime())
}

func StatFallback(path string, followSymlink bool) (fs.FileInfo, error) {
	if followSymlink {
		return os.Stat(path)
	}
	return os.Lstat(path)
}
