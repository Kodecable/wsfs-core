//go:build unix

package harness

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type unixPlatform struct{}

func NewPlatform(Config) (Platform, error) {
	return unixPlatform{}, nil
}

func (unixPlatform) PrepareEnv(env *Env) error {
	env.MountDir = filepath.Join(env.CaseRoot, "mount")
	env.MountArg = env.MountDir
	return os.MkdirAll(env.MountDir, 0o755)
}

func (unixPlatform) WaitMountReady(ctx context.Context, env *Env, proc *Process) error {
	for {
		if proc.Exited() {
			return fmt.Errorf("mount exited before ready, code=%d", ExitErrorCode(proc.WaitErr()))
		}
		if isMounted(env.MountDir) {
			if _, err := os.ReadDir(env.MountDir); err == nil {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (unixPlatform) Unmount(env *Env) error {
	if !isMounted(env.MountDir) {
		return nil
	}
	var errs []string
	for _, argv := range [][]string{
		{"fusermount3", "-u", env.MountDir},
		{"umount", env.MountDir},
	} {
		cmd := exec.Command(argv[0], argv[1:]...)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", strings.Join(argv, " "), err))
		}
	}
	return errors.New(strings.Join(errs, "; "))
}

func isMounted(target string) bool {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 5 && fields[4] == target {
			return true
		}
	}
	return false
}
