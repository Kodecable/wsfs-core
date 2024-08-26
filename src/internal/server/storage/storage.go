package storage

import (
	"path/filepath"
	"strings"
	"wsfs-core/internal/server/config"
)

type Storage struct {
	Path string // abdsouted, end with no '/'
}

func NewStorage(c *config.Storage) (s *Storage, err error) {
	s = &Storage{}

	s.Path, err = filepath.Abs(c.Path)
	if err != nil {
		return
	}
	s.Path = strings.TrimSuffix(s.Path, "/")

	return
}
