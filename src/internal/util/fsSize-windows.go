//go:build windows

package util

import "golang.org/x/sys/windows"

func FsSize(fspath string) (total, free, avail uint64, err error) {
	str, _ := windows.UTF16PtrFromString(fspath)
	err = windows.GetDiskFreeSpaceEx(str, &free, &total, &avail)
	return
}
