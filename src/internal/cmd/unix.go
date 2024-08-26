//go:build unix

package cmd

import (
	"wsfs-core/internal/cmd/mount"
	reloadserver "wsfs-core/internal/cmd/reload-server"
)

func init() {
	rootCmd.AddCommand(reloadserver.ReloadConfigCmd)
	rootCmd.AddCommand(mount.MountCmd)
}
