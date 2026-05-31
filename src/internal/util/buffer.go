package util

import (
	"io"
)

//const stringBufferInitalSize int = 64
//const nulChar byte = 0x00

type Buffer struct {
	Bytes   []byte
	writted int
}

func NewBuffer(size int) *Buffer {
	return &Buffer{
		Bytes: make([]byte, size),
	}
}

var _ = (io.Writer)((*Buffer)(nil))

func (b *Buffer) Write(d []byte) (n int, err error) {
	size := len(d)
	if avail := len(b.Bytes) - b.writted; size > avail {
		size = avail
		err = io.ErrShortWrite
	}
	n = copy(b.Bytes[b.writted:], d[:size])
	b.writted += n
	return
}

func (b *Buffer) Writted() int {
	return b.writted
}

func (b *Buffer) Done() []byte {
	n := b.writted
	b.writted = 0
	return b.Bytes[:n]
}

func (b *Buffer) Grow(n int) {
	b.writted += n
}

func (b *Buffer) Reset() {
	b.writted = 0
}
