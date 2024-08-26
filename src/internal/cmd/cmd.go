package cmd

import (
	"wsfs-core/internal/cmd/hash"
	"wsfs-core/internal/cmd/serve"
	"wsfs-core/internal/cmd/version"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wsfs",
	Short: "Mount or serve a Websocket Filesystem",
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(serve.ServeCmd)
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(hash.HashCmd)
}
