package webui

import (
	"os"
	"strings"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/server/webui/templates"

	"github.com/rs/zerolog/log"
)

type ListArg struct {
	Paths []string
	Files []templates.FileInfo
}

func list(rpath string, storage *storage.Storage) (l ListArg, err error) {
	apath := storage.Path + rpath
	l.Paths = strings.Split(rpath[:len(rpath)-1], "/")

	f, err := os.Open(apath)
	if err != nil {
		log.Warn().Err(err).Str("Path", apath).Msg("Open dir failed")
		return ListArg{}, err
	}
	defer f.Close()

	files, err := f.Readdir(-1)
	if err != nil {
		log.Warn().Err(err).Str("Path", apath).Msg("Read dir failed")
		return ListArg{}, err
	}

	for _, file := range files {
		// follow symlink
		realfile := file
		if file.Mode().Type() == os.ModeSymlink {
			realfile, err = os.Stat(apath + file.Name())
			if err != nil {
				log.Warn().Err(err).Str("Path", apath+file.Name()).Msg("Stat symlink failed")
				// show symlink itself instead
				realfile = file
				err = nil
			}
		}

		l.Files = append(l.Files, templates.FileInfo{
			Name:  file.Name(),
			IsDir: realfile.IsDir(),
			MTime: realfile.ModTime(),
			Size:  realfile.Size(),
		})
	}

	return
}
