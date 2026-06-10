//go:build !unix

package util

func GetDefaultFsIds() (FsIds, error) {
	return FsIds{}, nil
}
