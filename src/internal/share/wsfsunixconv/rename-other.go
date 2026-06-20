//go:build unix && !linux

package wsfsunixconv

var RenameFlagToUnix = map[uint32]uint32{}
