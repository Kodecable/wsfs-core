package harness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Config struct {
	WsfsBin       string
	WorkRoot      string
	StructTimeout int
	KeepWork      bool
	Verbose       bool
}

type Case interface {
	Name() string
	Prepare(*Env) error
	Run(ctx context.Context, env *Env) error
}

type Env struct {
	CaseName   string
	CaseRoot   string
	BackendDir string
	MountDir   string
	MountArg   string
	LogsDir    string
	ServerLog  string
	MountLog   string
	Endpoint   string
}

type Result struct {
	Name    string
	WorkDir string
	Err     error
}

type Runner struct {
	cfg      Config
	platform Platform
}

func NewRunner(cfg Config) (*Runner, error) {
	if cfg.WsfsBin == "" {
		return nil, errors.New("wsfs binary path is required")
	}
	if cfg.WorkRoot == "" {
		return nil, errors.New("work root is required")
	}
	if cfg.StructTimeout < 0 {
		cfg.StructTimeout = 0
	}
	absWorkRoot, err := filepath.Abs(cfg.WorkRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve work root: %w", err)
	}
	cfg.WorkRoot = absWorkRoot
	if err := os.MkdirAll(cfg.WorkRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create work root: %w", err)
	}
	platform, err := NewPlatform(cfg)
	if err != nil {
		return nil, err
	}
	return &Runner{cfg: cfg, platform: platform}, nil
}

func (r *Runner) RunCase(ctx context.Context, c Case) Result {
	res := Result{Name: c.Name()}

	caseRoot := filepath.Join(r.cfg.WorkRoot, sanitizeName(c.Name()))
	absCaseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		res.Err = fmt.Errorf("resolve case root: %w", err)
		return res
	}
	caseRoot = absCaseRoot
	_ = os.RemoveAll(caseRoot)
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		res.Err = fmt.Errorf("prepare case root: %w", err)
		return res
	}
	res.WorkDir = caseRoot

	env := &Env{
		CaseName:   c.Name(),
		CaseRoot:   caseRoot,
		BackendDir: filepath.Join(caseRoot, "backend"),
		LogsDir:    filepath.Join(caseRoot, "logs"),
		ServerLog:  filepath.Join(caseRoot, "logs", "server.log"),
		MountLog:   filepath.Join(caseRoot, "logs", "mount.log"),
	}
	for _, dir := range []string{env.BackendDir, env.LogsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			res.Err = fmt.Errorf("prepare work dir: %w", err)
			return res
		}
	}
	if err := r.platform.PrepareEnv(env); err != nil {
		res.Err = fmt.Errorf("prepare mount env: %w", err)
		return res
	}
	if err := c.Prepare(env); err != nil {
		res.Err = fmt.Errorf("prepare case: %w", err)
		return res
	}

	port, err := reserveTCPPort()
	if err != nil {
		res.Err = fmt.Errorf("reserve port: %w", err)
		return res
	}
	env.Endpoint = fmt.Sprintf("wsfs://127.0.0.1:%d/", port)

	serverProc, err := r.startServer(env, port)
	if err != nil {
		res.Err = err
		return res
	}

	var mountProc *Process
	defer func() {
		if cleanupErr := r.cleanup(env, mountProc, serverProc); cleanupErr != nil && res.Err == nil {
			res.Err = cleanupErr
		}
		if res.Err == nil && !r.cfg.KeepWork {
			_ = os.RemoveAll(caseRoot)
		}
	}()

	if err := waitTCPReady(ctx, port, serverProc); err != nil {
		res.Err = fmt.Errorf("wait server ready: %w", err)
		return res
	}

	mountProc, err = r.startMount(env)
	if err != nil {
		res.Err = err
		return res
	}

	if err := r.platform.WaitMountReady(ctx, env, mountProc); err != nil {
		res.Err = fmt.Errorf("wait mount ready: %w", err)
		return res
	}

	res.Err = c.Run(ctx, env)
	return res
}

func (r *Runner) cleanup(env *Env, mountProc, serverProc *Process) error {
	var errs []string

	if err := r.platform.Unmount(env); err != nil {
		errs = append(errs, fmt.Sprintf("unmount: %v", err))
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if mountProc != nil {
		if err := mountProc.Stop(stopCtx); err != nil && ExitErrorCode(err) != 0 {
			errs = append(errs, fmt.Sprintf("stop mount: %v", err))
		}
	}
	if serverProc != nil {
		if err := serverProc.Stop(stopCtx); err != nil && ExitErrorCode(err) != 0 {
			errs = append(errs, fmt.Sprintf("stop server: %v", err))
		}
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (r *Runner) startServer(env *Env, port int) (*Process, error) {
	cmd := exec.Command(
		r.cfg.WsfsBin,
		"quick-serve",
		"--storage", env.BackendDir,
		"--level", "debug",
		"--no-log-color",
		fmt.Sprintf("127.0.0.1:%d", port),
	)
	return StartProcess("server", env.ServerLog, cmd)
}

func (r *Runner) startMount(env *Env) (*Process, error) {
	cmd := exec.Command(
		r.cfg.WsfsBin,
		"mount",
		"--struct-timeout", fmt.Sprintf("%d", r.cfg.StructTimeout),
		"--level", "debug",
		"--no-log-color",
		env.Endpoint,
		env.MountArg,
	)
	return StartProcess("mount", env.MountLog, cmd)
}

func reserveTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitTCPReady(ctx context.Context, port int, proc *Process) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if proc.Exited() {
			return fmt.Errorf("server exited before ready, code=%d", ExitErrorCode(proc.WaitErr()))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func SortedNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func StatSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func ReadAt(path string, offset int64, size int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, size)
	n, err := f.ReadAt(buf, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buf[:n], nil
}
