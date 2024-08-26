//go:build unix

package reloadserver

import (
	"fmt"
	"os"
	"syscall"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/cobra"
)

var serverPid int32

func findProcess() (int32, error) {
	ps, err := process.Processes()
	if err != nil {
		return 0, err
	}

	selfPath, err := os.Executable()
	if err != nil {
		return 0, err
	}

	selfPid := int32(os.Getpid())

	for _, p := range ps {
		path, err := p.Exe()
		if err != nil {
			continue // pid 1 do hate this
		}

		if path == selfPath && p.Pid != selfPid {
			return p.Pid, nil
		}
	}

	return -1, nil
}

var ReloadConfigCmd = &cobra.Command{
	Use:   "reload-server",
	Short: "Tell a server reload config",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		var err error

		if serverPid <= 0 {
			serverPid, err = findProcess()

			if err != nil {
				fmt.Fprintf(os.Stderr, "Find server process failed: %e\n", err)
				os.Exit(2)
			}

			if serverPid <= 0 {
				fmt.Fprintf(os.Stderr, "No server found. Try specifying the pid manually.\n")
				os.Exit(1)
			}
		}

		p, err := process.NewProcess(serverPid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Send signal failed: %e\n", err)
			os.Exit(2)
		}

		err = p.SendSignal(syscall.SIGHUP)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Send signal failed: %e\n", err)
			os.Exit(2)
		}
	},
}

func init() {
	ReloadConfigCmd.Flags().Int32VarP(&serverPid, "pid", "p", 0, "Server pid (try find when 0 or negtive)")
}
