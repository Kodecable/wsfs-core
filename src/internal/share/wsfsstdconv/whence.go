package wsfsstdconv

import (
	"io"
	"wsfs-core/internal/share/wsfsprotocol"
)

var WhenceToStd = map[uint8]int{
	wsfsprotocol.WHENCE_SET: io.SeekStart,
	wsfsprotocol.WHENCE_CUR: io.SeekCurrent,
	wsfsprotocol.WHENCE_END: io.SeekEnd,
}

var WhenceFromStd = map[int]uint8{}

func init() {
	for protocol, platform := range WhenceToStd {
		WhenceFromStd[platform] = protocol
	}
}
