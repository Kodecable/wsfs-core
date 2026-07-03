package util

import (
	"io"
)

//const stringBufferInitalSize int = 64
//const nulChar byte = 0x00

type Buffer struct {
	Bytes   []byte
	written int
}

func NewBuffer(size int) *Buffer {
	return &Buffer{
		Bytes: make([]byte, size),
	}
}

var _ = (io.Writer)((*Buffer)(nil))

func (b *Buffer) Write(d []byte) (n int, err error) {
	size := len(d)
	if avail := len(b.Bytes) - b.written; size > avail {
		size = avail
		err = io.ErrShortWrite
	}
	n = copy(b.Bytes[b.written:], d[:size])
	b.written += n
	return
}

func (b *Buffer) Written() int {
	return b.written
}

func (b *Buffer) Done() []byte {
	n := b.written
	b.written = 0
	return b.Bytes[:n]
}

func (b *Buffer) Grow(n int) {
	b.written += n
}

func (b *Buffer) Reset() {
	b.written = 0
}
