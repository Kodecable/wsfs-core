package config

import (
	"errors"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var ErrReDecodeDefaultConfig = errors.New("can not redecode default config")

func Decode(config *Server, path string) error {
	_, err := toml.DecodeFile(path, config)
	if err != nil {
		return err
	}

	config.filePath, err = filepath.Abs(path)
	return err
}

func ReDecode(config *Server, old *Server) error {
	if old.filePath == Default.filePath {
		return ErrReDecodeDefaultConfig
	}

	return Decode(config, old.filePath)
}
