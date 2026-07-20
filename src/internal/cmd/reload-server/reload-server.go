//go:build unix

package reloadserver

import (
	"fmt"
	"os"
	"syscall"
	cmdexit "wsfs-core/internal/cmd/exit"

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
	RunE: func(c *cobra.Command, _ []string) error {
		var err error

		if !c.Flags().Changed("pid") {
			serverPid, err = findProcess()

			if err != nil {
				return cmdexit.New(2, fmt.Errorf("find server process failed: %w", err))
			}

			if serverPid <= 0 {
				return cmdexit.New(1, fmt.Errorf("no server found; try specifying the pid manually"))
			}
		}

		p, err := process.NewProcess(serverPid)
		if err != nil {
			return cmdexit.New(2, fmt.Errorf("send signal failed: %w", err))
		}

		err = p.SendSignal(syscall.SIGHUP)
		if err != nil {
			return cmdexit.New(2, fmt.Errorf("send signal failed: %w", err))
		}
		return nil
	},
}

func init() {
	ReloadConfigCmd.Flags().Int32VarP(&serverPid, "pid", "p", 0, "Server pid")
}
