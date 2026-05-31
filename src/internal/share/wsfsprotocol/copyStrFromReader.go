package wsfsprotocol

import (
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
	var b [1]byte
	var sb = stringBuilderPool.Get().(*strings.Builder)
	defer stringBuilderPool.Put(sb)

	for {
		n, err := io.ReadFull(r, b[:])
		if n != 0 {
			if b[0] == 0 {
				*s = sb.String()
				sb.Reset()
				return err
			}
			sb.WriteByte(b[0])
		}
		if err != nil {
			sb.Reset()
			return err
		}
	}
}
