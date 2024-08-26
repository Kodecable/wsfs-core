package version

import (
	"fmt"
	"runtime"
	"wsfs-core/buildinfo"

	"github.com/spf13/cobra"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version infomation",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("WSFS %s %s\n", buildinfo.Version, buildinfo.Mode)
		fmt.Printf("Build   %s\n", buildinfo.Time)
		fmt.Printf("Arch    %s\n", runtime.GOARCH)
		fmt.Printf("OS      %s\n", runtime.GOOS)
	},
}
