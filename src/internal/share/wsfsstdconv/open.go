package wsfsstdconv

import (
	"os"
	"wsfs-core/internal/share/wsfsprotocol"
)

var OpenFlagToStd = map[uint32]int{
	wsfsprotocol.O_RDONLY: os.O_RDONLY,
	wsfsprotocol.O_WRONLY: os.O_WRONLY,
	wsfsprotocol.O_RDWR:   os.O_RDWR,
	wsfsprotocol.O_TRUNC:  os.O_TRUNC,
	wsfsprotocol.O_EXCL:   os.O_EXCL,
	wsfsprotocol.O_APPEND: os.O_APPEND,
}

var OpenFlagFromStd = map[int]uint32{}

func init() {
	for protocol, platform := range OpenFlagToStd {
		OpenFlagFromStd[platform] = protocol
	}
}
