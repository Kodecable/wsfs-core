//go:build windows

package cmd

import (
	"wsfs-core/internal/cmd/mount"
)

func init() {
	rootCmd.AddCommand(mount.MountCmd)
}
