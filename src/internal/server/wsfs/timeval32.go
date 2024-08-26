//go:build arm || 386 || mips || mipsle

package wsfs

func timeval(v int64) int32 {
	return int32(v)
}
