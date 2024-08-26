package util

import (
	"encoding/binary"
	"io"
	"strings"
	"sync"
)

var stringBuilderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

func CopyStrFromReader(r io.Reader, s *string) (err error) {
	var u byte
	var sb = stringBuilderPool.Get().(*strings.Builder)

	for {
		err = binary.Read(r, binary.LittleEndian, &u)
		if err != nil {
			sb.Reset()
			return err
		}
		if u == 0 {
			*s = sb.String()
			sb.Reset()
			return
		}

		sb.WriteByte(u)
	}
}
