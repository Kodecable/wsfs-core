//go:build !windows && !unix

package wsfs

import (
	"io/fs"
	"os"
)

func openSFD(name string, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}
