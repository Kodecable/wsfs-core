package harness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Process struct {
	Name    string
	Cmd     *exec.Cmd
	LogPath string

	done chan struct{}

	waitErr error
}

func StartProcess(name string, logPath string, cmd *exec.Cmd) (*Process, error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("create log file for %s: %w", name, err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	p := &Process{
		Name:    name,
		Cmd:     cmd,
		LogPath: logPath,
		done:    make(chan struct{}),
	}

	go func() {
		p.waitErr = cmd.Wait()
		_ = logFile.Close()
		close(p.done)
	}()

	return p, nil
}

func (p *Process) PID() int {
	if p == nil || p.Cmd == nil || p.Cmd.Process == nil {
		return -1
	}
	return p.Cmd.Process.Pid
}

func (p *Process) Exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *Process) WaitErr() error {
	if p == nil {
		return nil
	}
	<-p.done
	return p.waitErr
}

func (p *Process) Signal(sig os.Signal) error {
	if p == nil || p.Cmd == nil || p.Cmd.Process == nil {
		return nil
	}
	return p.Cmd.Process.Signal(sig)
}

func (p *Process) Stop(ctx context.Context) error {
	if p == nil || p.Cmd == nil || p.Cmd.Process == nil {
		return nil
	}
	if p.Exited() {
		return p.WaitErr()
	}

	_ = p.Signal(os.Interrupt)

	select {
	case <-p.done:
		return p.waitErr
	case <-ctx.Done():
	}

	_ = p.Signal(syscall.SIGTERM)
	select {
	case <-p.done:
		return p.waitErr
	case <-time.After(2 * time.Second):
	}

	_ = p.Cmd.Process.Kill()
	select {
	case <-p.done:
		return p.waitErr
	default:
		return nil
	}
}

func CopyTail(w io.Writer, path string, limit int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	offset := int64(0)
	if info.Size() > limit {
		offset = info.Size() - limit
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	if offset != 0 {
		if _, err := io.WriteString(w, "...<tail truncated>...\n"); err != nil {
			return err
		}
	}

	_, err = io.Copy(w, f)
	return err
}

func ExitErrorCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if err == nil {
		return 0
	}
	return -1
}
