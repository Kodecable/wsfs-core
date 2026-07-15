//go:build !linux && !darwin

package xattr

import "syscall"

func Set(string, string, []byte, uint32) error {
	return syscall.ENOTSUP
}

func Get(string, string, uint32) ([]byte, error) {
	return nil, syscall.ENOTSUP
}

func List(string, uint32) ([]byte, error) {
	return nil, syscall.ENOTSUP
}

func Remove(string, string, uint32) error {
	return syscall.ENOTSUP
}

func IsNoXAttr(error) bool {
	return false
}
