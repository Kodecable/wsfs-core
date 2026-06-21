//go:build !(linux && (386 || arm || mips || mipsle || ppc))

package timeval

import (
	"io/fs"

	"wsfs-core/internal/share/wsfsprotocol"
)

func MTimeFromFileInfo(fi fs.FileInfo) wsfsprotocol.Timespec {
	return FromFileInfo(fi)
}

func Stat(path string, followSymlink bool) (fs.FileInfo, wsfsprotocol.Timespec, error) {
	fi, err := StatFallback(path, followSymlink)
	if err != nil {
		return nil, wsfsprotocol.Timespec{}, err
	}
	return fi, MTimeFromFileInfo(fi), nil
}
