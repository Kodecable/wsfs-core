//go:build arm64 || amd64 || s390x || loong64 || riscv64 || ppc64 || ppc64le || mips64 || mips64le

package wsfs

func timeval(v int64) int64 {
	return v
}
