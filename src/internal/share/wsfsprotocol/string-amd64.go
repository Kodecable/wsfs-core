//go:build amd64

package wsfsprotocol

import (
	"io"
	"unsafe"
)

func writeStrLenPrefix(length uint16, w io.Writer) error {
	_, err := w.Write(unsafe.Slice((*byte)(unsafe.Pointer(&length)), 2))
	return err
}

func readStrLenPrefix(r io.Reader) (length uint16, err error) {
	_, err = io.ReadFull(r, unsafe.Slice((*byte)(unsafe.Pointer(&length)), 2))
	return
}
