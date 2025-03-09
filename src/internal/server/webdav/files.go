package webdav

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// moveFiles moves files and/or directories from src to dst.
//
// See section 9.9.4 for when various HTTP status codes apply.
func moveFiles(src, dst string, overwrite bool) (status int, err error) {
	created := false
	if _, err := os.Stat(dst); err != nil {
		if !os.IsNotExist(err) {
			return http.StatusForbidden, err
		}
		created = true
	} else if overwrite {
		// Section 9.9.3 says that "If a resource exists at the destination
		// and the Overwrite header is "T", then prior to performing the move,
		// the server must perform a DELETE with "Depth: infinity" on the
		// destination resource.
		if err := os.RemoveAll(dst); err != nil {
			return http.StatusForbidden, err
		}
	} else {
		return http.StatusPreconditionFailed, os.ErrExist
	}
	if err := os.Rename(src, dst); err != nil {
		return http.StatusForbidden, err
	}
	if created {
		return http.StatusCreated, nil
	}
	return http.StatusNoContent, nil
}

// copyFiles copies files and/or directories from src to dst.
//
// See section 9.8.5 for when various HTTP status codes apply.
func copyFiles(src, dst string, overwrite bool, depth int, recursion int) (status int, err error) {
	if recursion >= recursionMax {
		return http.StatusInternalServerError, errRecursionTooDeep
	}
	recursion++

	// TODO: section 9.8.3 says that "Note that an infinite-depth COPY of /A/
	// into /A/B/ could lead to infinite recursion if not handled correctly."

	srcFile, err := os.OpenFile(src, os.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return http.StatusNotFound, err
		}
		return http.StatusInternalServerError, err
	}
	defer srcFile.Close()
	srcStat, err := srcFile.Stat()
	if err != nil {
		if os.IsNotExist(err) {
			return http.StatusNotFound, err
		}
		return http.StatusInternalServerError, err
	}
	srcPerm := srcStat.Mode() & os.ModePerm

	created := false
	if _, err := os.Stat(dst); err != nil {
		if os.IsNotExist(err) {
			created = true
		} else {
			return http.StatusForbidden, err
		}
	} else {
		if !overwrite {
			return http.StatusPreconditionFailed, os.ErrExist
		}
		if err := os.RemoveAll(dst); err != nil && !os.IsNotExist(err) {
			return http.StatusForbidden, err
		}
	}

	if srcStat.IsDir() {
		if err := os.Mkdir(dst, srcPerm); err != nil {
			return http.StatusForbidden, err
		}
		if depth == infiniteDepth {
			children, err := srcFile.Readdir(-1)
			if err != nil {
				return http.StatusForbidden, err
			}
			for _, c := range children {
				name := c.Name()
				s := path.Join(src, name)
				d := path.Join(dst, name)
				cStatus, cErr := copyFiles(s, d, overwrite, depth, recursion)
				if cErr != nil {
					// TODO: MultiStatus.
					return cStatus, cErr
				}
			}
		}

	} else {
		dstFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcPerm)
		if err != nil {
			if os.IsNotExist(err) {
				return http.StatusConflict, err
			}
			return http.StatusForbidden, err

		}
		_, copyErr := io.Copy(dstFile, srcFile)
		closeErr := dstFile.Close()
		if copyErr != nil {
			return http.StatusInternalServerError, copyErr
		}
		if closeErr != nil {
			return http.StatusInternalServerError, closeErr
		}
	}

	if created {
		return http.StatusCreated, nil
	}
	return http.StatusNoContent, nil
}

// walkFS traverses filesystem fs starting at name up to depth levels.
//
// Allowed values for depth are 0, 1 or infiniteDepth. For each visited node,
// walkFS calls walkFn. If there is an error, walkFS calls walkFn with error.
// For each node, walkFn will be called only once.
func walkFS(depth int, base, path_ string, info os.FileInfo, walkFn filepath.WalkFunc) error {
	// This implementation is based on Walk's code in the standard path/filepath package.
	if err := walkFn(path_, info, nil); err != nil {
		return err
	}

	if depth == 0 {
		return nil
	}

	fullpath := path.Join(base, path_)
	// Read directory names.
	f, err := os.OpenFile(fullpath, os.O_RDONLY, 0)
	if err != nil {
		walkFn(path_, info, err)
		return err
	}
	fileInfos, err := f.Readdir(0)
	f.Close()
	if err != nil {
		walkFn(path_, info, err)
		return err
	}

	for _, fi := range fileInfos {
		passfi := fi
		if fi.Mode()&fs.ModeSymlink != 0 {
			if realfi, err := os.Stat(path.Join(fullpath, fi.Name())); err == nil {
				passfi = realfi
			} else {
				log.Warn().Err(err).Str("Path", path.Join(fullpath, fi.Name())).Msg("follow symlink failed")
			}
		}
		if fi.IsDir() {
			walkFS(depth-1, base, path.Join(path_, fi.Name()), passfi, walkFn)
		} else {
			walkFn(path.Join(path_, fi.Name()), passfi, nil)
		}
	}

	return nil
}
