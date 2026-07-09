//go:build windows

package wsfs

import (
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

func openSFD(name string, flag int, perm fs.FileMode) (*os.File, error) {
	if name == "" {
		return nil, syscall.ERROR_FILE_NOT_FOUND
	}
	namep, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}

	accessFlags := flag & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR)
	var access uint32
	switch accessFlags {
	case os.O_RDONLY:
		access = windows.GENERIC_READ
	case os.O_WRONLY:
		access = windows.GENERIC_WRITE
	case os.O_RDWR:
		access = windows.GENERIC_READ | windows.GENERIC_WRITE
	}
	if flag&os.O_CREATE != 0 {
		access |= windows.GENERIC_WRITE
	}
	if flag&os.O_APPEND != 0 {
		if flag&os.O_TRUNC == 0 {
			access &^= windows.GENERIC_WRITE
		}
		access |= windows.FILE_GENERIC_WRITE &^ windows.FILE_WRITE_DATA
	}

	sharemode := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	attrs := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	if perm&0200 == 0 {
		attrs = windows.FILE_ATTRIBUTE_READONLY
	}
	if accessFlags == os.O_RDONLY {
		attrs |= windows.FILE_FLAG_BACKUP_SEMANTICS
	}
	if flag&os.O_SYNC != 0 {
		attrs |= windows.FILE_FLAG_WRITE_THROUGH
	}

	var createmode uint32
	switch {
	case flag&(os.O_CREATE|os.O_EXCL) == os.O_CREATE|os.O_EXCL:
		createmode = windows.CREATE_NEW
		attrs |= windows.FILE_FLAG_OPEN_REPARSE_POINT
	case flag&os.O_CREATE != 0:
		createmode = windows.OPEN_ALWAYS
	default:
		createmode = windows.OPEN_EXISTING
	}

	h, err := windows.CreateFile(namep, access, sharemode, nil, createmode, attrs, 0)
	if err != nil {
		if err == windows.ERROR_ACCESS_DENIED && attrs&windows.FILE_FLAG_BACKUP_SEMANTICS == 0 {
			if fa, e1 := windows.GetFileAttributes(namep); e1 == nil && fa&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
				err = syscall.EISDIR
			}
		}
		return nil, err
	}

	if flag&os.O_TRUNC != 0 && (createmode == windows.OPEN_EXISTING || createmode == windows.OPEN_ALWAYS) {
		err = windows.Ftruncate(h, 0)
		if err == windows.ERROR_INVALID_PARAMETER {
			if t, err1 := windows.GetFileType(h); err1 == nil &&
				(t == windows.FILE_TYPE_PIPE || t == windows.FILE_TYPE_CHAR) {
				err = nil
			}
		}
		if err != nil {
			_ = windows.CloseHandle(h)
			return nil, err
		}
	}

	return os.NewFile(uintptr(h), name), nil
}
