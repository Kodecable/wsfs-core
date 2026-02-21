//go:build !windows

package util

func isUrlValid_os(string) bool {
	return true
}
