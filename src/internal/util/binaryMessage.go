package util

import (
	"encoding/binary"
	"io"
	"strings"
)

//const stringBufferInitalSize int = 64
//const nulChar byte = 0x00

type Buffer struct {
	buf []byte
	len int
}

func NewBuffer(size int) *Buffer {
	return &Buffer{
		buf: make([]byte, size),
	}
}

func (b *Buffer) Done() []byte {
	len := b.len
	b.len = 0
	return b.buf[:len]
}

func (b *Buffer) Put(v any) {
	switch v := v.(type) {
	case []byte:
		copy(b.buf[b.len:], v)
		b.len += len(v)
	case string:
		copy(b.buf[b.len:], v)
		b.buf[b.len+len(v)] = 0
		b.len += len(v) + 1
	case uint8:
		b.buf[b.len] = v
		b.len += 1
	case uint16:
		binary.LittleEndian.PutUint16(b.buf[b.len:], v)
		b.len += 2
	case uint32:
		binary.LittleEndian.PutUint32(b.buf[b.len:], v)
		b.len += 4
	case uint64:
		binary.LittleEndian.PutUint64(b.buf[b.len:], v)
		b.len += 8
	case int8:
		b.buf[b.len] = uint8(v)
		b.len += 1
	case int16:
		binary.LittleEndian.PutUint16(b.buf[b.len:], uint16(v))
		b.len += 2
	case int32:
		binary.LittleEndian.PutUint32(b.buf[b.len:], uint32(v))
		b.len += 4
	case int64:
		binary.LittleEndian.PutUint64(b.buf[b.len:], uint64(v))
		b.len += 8
	}
}

func (b *Buffer) DirectPutStart(maxSize int) []byte {
	return b.buf[b.len : b.len+maxSize]
}

func (b *Buffer) DirectPutDone(size int) {
	b.len += size
}

var _ = (io.Writer)((*Buffer)(nil))

func (b *Buffer) Write(d []byte) (int, error) {
	size := b.len
	b.len += len(d)
	return copy(b.buf[size:], d), nil
}

func (b *Buffer) Ensure(off int) bool {
	return off < b.len
}

func (b *Buffer) ReadByteAt(off int) byte {
	return b.buf[off]
}

func (b *Buffer) ReadData(off int) []byte {
	return b.buf[off:b.len]
}

func (b *Buffer) ReadString(off int) (str string, ok bool) {
	var u byte
	var sb = stringBuilderPool.Get().(*strings.Builder)

	for {
		u = b.buf[off]
		if u == 0 {
			break
		}
		sb.WriteByte(u)

		off++
		if off >= b.len {
			sb.Reset()
			stringBuilderPool.Put(sb)
			return "", false
		}
	}

	s := sb.String()
	sb.Reset()
	stringBuilderPool.Put(sb)
	return s, true
}

func (b *Buffer) ReadU16(off int) uint16 {
	return binary.LittleEndian.Uint16(b.buf[off:])
}

func (b *Buffer) ReadU32(off int) uint32 {
	return binary.LittleEndian.Uint32(b.buf[off:])
}

func (b *Buffer) ReadU64(off int) uint64 {
	return binary.LittleEndian.Uint64(b.buf[off:])
}
