//go:build !unix && !windows

package util

func FsSize(fspath string) (total, free, avail uint64, err error) {
	//fake data
	return uint64(10995116277760),
		uint64(5497558138880),
		uint64(5497558138880),
		nil
}
