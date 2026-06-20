//go:build unix

package wsfsunixconv

var WhenceFromUnix = map[int]uint8{}

func init() {
	for protocol, platform := range WhenceToUnix {
		WhenceFromUnix[platform] = protocol
	}
}
