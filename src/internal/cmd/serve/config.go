package serve

import (
	"fmt"
	"os"
	serverConfig "wsfs-core/internal/server/config"
	"wsfs-core/internal/util"
)

var defaultConfigPaths = []string{
	"server.toml",
	"wsfs.toml",
	"config.toml",
	"~/.config/wsfs/server.toml",
	"/etc/wsfs/server.toml",
}

const iternalDefaultConfigPath = "<DEFAULT>"

func findConfigFile() string {
	for _, path := range defaultConfigPaths {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Found config file: %s\n", path)
			return path
		}
	}

	return iternalDefaultConfigPath
}

func findAndDecodeConfig() *serverConfig.Server {
	config := serverConfig.Default

	if configPath == iternalDefaultConfigPath {
		fmt.Println("No config file given, finding...")
		configPath = findConfigFile()
	}

	if configPath != iternalDefaultConfigPath {
		err := serverConfig.Decode(&config, configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Decode config file failed")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	} else {
		fmt.Println("No config file found. Configed as all default.")
	}

	setUids(&config)
	return &config
}

func setUids(c *serverConfig.Server) {
	if c.Uid >= 0 && c.Gid >= 0 &&
		c.OtherUid >= 0 && c.OtherGid >= 0 {
		return
	}

	defaultIds, err := util.GetDefaultIds()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to determine default (nobody) u/gids")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if c.Uid < 0 {
		c.Uid = int64(defaultIds.CurrentUser)
	}
	if c.Gid < 0 {
		c.Gid = int64(defaultIds.UserGroup)
	}
	if c.OtherUid < 0 {
		c.OtherUid = int64(defaultIds.NobodyUser)
	}
	if c.OtherGid < 0 {
		c.OtherGid = int64(defaultIds.NobodyGroup)
	}
}
