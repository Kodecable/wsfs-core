package cmd

import (
	"wsfs-core/internal/cmd/exit"
	"wsfs-core/internal/cmd/hash"
	quickserve "wsfs-core/internal/cmd/quick-serve"
	"wsfs-core/internal/cmd/serve"
	"wsfs-core/internal/cmd/version"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "wsfs",
	Short:         "Mount or serve a Websocket Filesystem",
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func ExitCode(err error) int {
	return exit.Code(err)
}

func init() {
	rootCmd.AddCommand(serve.ServeCmd)
	rootCmd.AddCommand(quickserve.QuickServeCmd)
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(hash.HashCmd)
}
