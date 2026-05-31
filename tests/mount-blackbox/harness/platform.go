package harness

import "context"

type Platform interface {
	PrepareEnv(env *Env) error
	WaitMountReady(ctx context.Context, env *Env, proc *Process) error
	Unmount(env *Env) error
}
