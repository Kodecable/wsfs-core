//go:build unix

package harness

import (
	"os/exec"
	"syscall"
)

func prepareMountCommand(cmd *exec.Cmd) {
}

func requestMountShutdown(proc *Process) error {
	return proc.Signal(syscall.SIGHUP)
}
