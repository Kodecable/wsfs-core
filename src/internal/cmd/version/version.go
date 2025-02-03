package version

import (
	"fmt"
	"os"
	"runtime"
	"wsfs-core/internal/util"
	"wsfs-core/version"

	"github.com/spf13/cobra"
)

var (
	all bool
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version infomation",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("WSFS %s %s\n", version.Version, version.Mode)
		fmt.Printf("Build     %s\n", version.Time)
		fmt.Printf("Platform  %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Go        %s\n", version.GoVersion())

		if all {
			fmt.Printf("BuildDeps:\n")
			table := util.NewTable().WithLeftPadding(2)
			for _, dep := range version.BuildDeps() {
				if dep.Path == "wsfs-core" {
					continue
				}
				table.AddRow(dep.Path, dep.Version)
			}
			table.Print(os.Stdout)

			fmt.Printf("BuildSettings:\n")
			table = util.NewTable().WithLeftPadding(2)
			for _, setting := range version.BuildSettings() {
				table.AddRow(setting.Key, setting.Value)
			}
			table.Print(os.Stdout)
		}
	},
}

func init() {
	VersionCmd.Flags().BoolVarP(&all, "all", "a", false, "More technical info")
}
