package version

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"wsfs-core/internal/util"
	"wsfs-core/version"

	"github.com/spf13/cobra"
)

var (
	all bool
)

const (
	readBuildInfoFailedMessage = "Unknown (Error: ReadBuildInfo() failed)"
)

func printDependencies(info *debug.BuildInfo, ok bool) {
	fmt.Printf("Build dependencies:\n")
	if !ok {
		fmt.Printf("  %s\n", readBuildInfoFailedMessage)
		return
	}

	table := util.NewTable().WithLeftPadding(2)
	for _, dep := range info.Deps {
		if dep.Path == "wsfs-core" {
			continue
		}
		module := dep
		for module.Replace != nil {
			module = module.Replace
		}

		if dep.Replace != nil {
			table.AddRow(dep.Path, dep.Version, "replaced by", module.Path, module.Version)
		} else {
			table.AddRow(dep.Path, dep.Version)
		}

	}
	table.Print(os.Stdout)
}

func printSettsings(info *debug.BuildInfo, ok bool) {
	fmt.Printf("Build settings:\n")
	if !ok {
		fmt.Printf("  %s\n", readBuildInfoFailedMessage)
		return
	}

	table := util.NewTable().WithLeftPadding(2)
	for _, setting := range info.Settings {
		if strings.HasPrefix(setting.Key, "vcs") ||
			setting.Key == "GOOS" || setting.Key == "GOARCH" {
			continue
		}
		table.AddRow(setting.Key, setting.Value)
	}
	table.Print(os.Stdout)
}

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version infomation",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		bi, ok := debug.ReadBuildInfo()
		goVersion := "Unknown (Error: ReadBuildInfo() failed)"
		if ok {
			goVersion, _ = strings.CutPrefix(bi.GoVersion, "go")
		}

		fmt.Printf("WSFS %s %s\n", version.Version, version.Mode)
		fmt.Printf("Build   %s\n", version.Time)
		fmt.Printf("Target  %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Go      %s\n", goVersion)

		if all {
			printDependencies(bi, ok)
			printSettsings(bi, ok)
		}
	},
}

func init() {
	VersionCmd.Flags().BoolVarP(&all, "all", "a", false, "more technical info")
}
