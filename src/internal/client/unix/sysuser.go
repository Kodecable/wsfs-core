//go:build unix

package unix

type Suser_t struct {
	Uid       uint32
	Gid       uint32
	NobodyUid uint32
	NobodyGid uint32
}
