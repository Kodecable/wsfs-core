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

func findConfigFile() string {
	for _, path_ := range defaultConfigPaths {
		if cpath, found := strings.CutPrefix(path_, "~/"); found {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Unable to get user home dir")
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			path_ = path.Join(homeDir, cpath)
		}

		if _, err := os.Stat(path_); err == nil {
			fmt.Printf("Found config file: %s\n", path_)
			return path_
		}
	}

	return internalDefaultConfigPath
}

func findAndDecodeConfig() serverConfig.Server {
	config := serverConfig.Default

	if configPath == internalDefaultConfigPath {
		fmt.Println("No config file given, finding...")
		configPath = findConfigFile()
	}

	if configPath != internalDefaultConfigPath {
		err := serverConfig.Decode(&config, configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Decode config file failed")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	} else {
		fmt.Println("No config file found. Configured as all default.")
	}

	return config
}
