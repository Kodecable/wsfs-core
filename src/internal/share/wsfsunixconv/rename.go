//go:build unix

package wsfsunixconv

var (
	RenameFlagFromUnix      = map[uint32]uint32{}
	AcceptedUnixRenameFlags uint32
	AcceptedWSFSRenameFlags uint32
)

func init() {
	for protocol, platform := range RenameFlagToUnix {
		RenameFlagFromUnix[platform] = protocol
		AcceptedUnixRenameFlags |= platform
		AcceptedWSFSRenameFlags |= protocol
	}
}
