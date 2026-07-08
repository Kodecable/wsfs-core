//go:build windows

package harness

import (
	"context"
	"fmt"
	"os"
	"time"
)

type windowsPlatform struct{}

func NewPlatform(Config) (Platform, error) {
	return windowsPlatform{}, nil
}

func (windowsPlatform) PrepareEnv(env *Env) error {
	drive, err := reserveDriveLetter()
	if err != nil {
		return err
	}
	env.MountArg = drive + ":"
	env.MountRoot = drive + ":\\"
	env.MountDir = env.MountRoot
	return nil
}

func (windowsPlatform) WaitMountReady(ctx context.Context, env *Env, proc *Process) error {
	for {
		if proc.Exited() {
			return fmt.Errorf("mount exited before ready, code=%d", ExitErrorCode(proc.WaitErr()))
		}
		if _, err := os.Stat(env.MountRoot); err == nil {
			if _, err := os.ReadDir(env.MountRoot); err == nil {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (windowsPlatform) Unmount(env *Env) error {
	// On Windows the mount process owns the lifetime of the WinFsp mount.
	// Runner cleanup stops the process afterwards, so there is nothing
	// reliable to do here before that point.
	return nil
}

func reserveDriveLetter() (string, error) {
	for letter := 'Z'; letter >= 'M'; letter-- {
		root := string(letter) + ":\\"
		if _, err := os.Stat(root); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			continue
		}
		return string(letter), nil
	}
	return "", fmt.Errorf("no free drive letter available")
}
