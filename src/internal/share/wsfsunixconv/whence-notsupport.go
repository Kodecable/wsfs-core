//go:build aix || netbsd || dragonfly || solaris

package wsfsunixconv

var WhenceToUnix = map[uint8]int{}

// TODO: check why no unix.SEEK_*
