//go:build !unix

package util

func GetDefaultIds() (IdInfo, error) {
	return IdInfo{0, 0, 0, 0}, nil
}
