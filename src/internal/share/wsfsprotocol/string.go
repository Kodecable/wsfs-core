package wsfsprotocol

import (
	"errors"
	"io"
	"unsafe"
)

var ErrStringTooLong = errors.New("wsfsprotocol: string too long")

func ReadStrFromReader(r io.Reader, s *string) (err error) {
	size, err := readStrLenPrefix(r)
	if err != nil {
		return err
	}

	if size == 0 {
		*s = ""
		return nil
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	*s = unsafe.String(unsafe.SliceData(buf), len(buf))

	return nil
}

func WriteStrToWriter(s string, w io.Writer) error {
	if len(s) > 0xffff {
		return ErrStringTooLong
	}
	if err := writeStrLenPrefix(uint16(len(s)), w); err != nil {
		return err
	}

	if len(s) == 0 {
		return nil
	}

	// if sw, ok := w.(io.StringWriter); ok {
	// 	_, err := sw.WriteString(s)
	// 	return  err
	// }
	_, err := w.Write(unsafe.Slice(unsafe.StringData(s), len(s)))
	return err
}
