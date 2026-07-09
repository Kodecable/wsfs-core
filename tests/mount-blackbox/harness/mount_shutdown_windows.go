//go:build windows

package harness

import (
	"fmt"
	"os/exec"
	"syscall"
)

func prepareMountCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func requestMountShutdown(proc *Process) error {
	if proc == nil || proc.Cmd == nil || proc.Cmd.Process == nil {
		return nil
	}
	kernel32, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return fmt.Errorf("load kernel32: %w", err)
	}
	defer kernel32.Release()

	generateConsoleCtrlEvent, err := kernel32.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return fmt.Errorf("find GenerateConsoleCtrlEvent: %w", err)
	}
	r1, _, callErr := generateConsoleCtrlEvent.Call(
		uintptr(syscall.CTRL_BREAK_EVENT),
		uintptr(proc.PID()),
	)
	if r1 == 0 {
		return fmt.Errorf("send CTRL_BREAK_EVENT to pid %d: %w", proc.PID(), callErr)
	}
	return nil
}
