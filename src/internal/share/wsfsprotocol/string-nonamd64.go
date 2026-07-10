//go:build !amd64

package wsfsprotocol

import (
	"encoding/binary"
	"io"
)

func writeStrLenPrefix(length uint16, w io.Writer) error {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], uint16(length))
	_, err := w.Write(buf[:])
	return err
}

func readStrLenPrefix(r io.Reader) (length uint16, err error) {
	var buf [2]byte
	if _, err = io.ReadFull(r, buf[:]); err != nil {
		return
	}
	length = (uint16)(binary.LittleEndian.Uint16(buf[0:]))
	return
}
