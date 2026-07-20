package serve

import (
	"fmt"
	"os"
	"path"
	"strings"
	serverConfig "wsfs-core/internal/server/config"
)

var defaultConfigPaths = []string{
	"server.toml",
	"wsfs.toml",
	"config.toml",
	"~/.config/wsfs/server.toml",
	"/etc/wsfs/server.toml",
}

const internalDefaultConfigPath = "<DEFAULT>"

func findConfigFile() (string, error) {
	for _, path_ := range defaultConfigPaths {
		if cpath, found := strings.CutPrefix(path_, "~/"); found {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("unable to get user home dir: %w", err)
			}
			path_ = path.Join(homeDir, cpath)
		}

		if _, err := os.Stat(path_); err == nil {
			fmt.Printf("Found config file: %s\n", path_)
			return path_, nil
		}
	}

	return internalDefaultConfigPath, nil
}

func findAndDecodeConfig(configPath string) (serverConfig.Server, error) {
	config := serverConfig.Default

	if configPath == internalDefaultConfigPath {
		fmt.Println("No config file given, finding...")
		var err error
		configPath, err = findConfigFile()
		if err != nil {
			return config, err
		}
	}

	if configPath != internalDefaultConfigPath {
		err := serverConfig.Decode(&config, configPath)
		if err != nil {
			return config, fmt.Errorf("decode config file failed: %w", err)
		}
	} else {
		fmt.Println("No config file found. Configured as all default.")
	}

	return config, nil
}
